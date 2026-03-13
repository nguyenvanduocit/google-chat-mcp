package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

type Config struct {
	CredentialsFile string
	Scopes          []string
	SessionDir      string
	BaseURL         string
}

type Server struct {
	mux         *http.ServeMux
	mcpServer   *server.StreamableHTTPServer
	googleOAuth *GoogleOAuth
	store       *SessionStore
	baseURL     string
}

func NewServer(mcpServer *server.MCPServer, cfg Config) (*Server, error) {
	store, err := NewSessionStore(cfg.SessionDir)
	if err != nil {
		return nil, fmt.Errorf("create session store: %w", err)
	}

	googleOAuth, err := NewGoogleOAuth(cfg.CredentialsFile, cfg.Scopes, store, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("create google oauth: %w", err)
	}

	s := &Server{
		mux:         http.NewServeMux(),
		googleOAuth: googleOAuth,
		store:       store,
		baseURL:     cfg.BaseURL,
	}

	// Create the StreamableHTTP server.
	s.mcpServer = server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
	)

	s.mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	s.mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleAuthServerMetadata)
	s.mux.HandleFunc("GET /authorize", googleOAuth.HandleAuthorize)
	s.mux.HandleFunc("GET /oauth/callback", googleOAuth.HandleCallback)
	s.mux.HandleFunc("POST /token", googleOAuth.HandleToken)
	s.mux.HandleFunc("POST /revoke", googleOAuth.HandleRevoke)
	s.mux.HandleFunc("/mcp", s.handleMCP)

	return s, nil
}

func (s *Server) Start(addr string) error {
	stop := make(chan struct{})
	s.store.StartCleanup(stop)

	srv := &http.Server{
		Addr:        addr,
		Handler:     s.mux,
		ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	err := srv.ListenAndServe()
	close(stop)
	return err
}

// handleMCP wraps the MCP handler with Bearer token authentication.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(
			`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`,
			s.baseURL,
		))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	session, err := s.store.GetSession(accessToken)
	if err != nil {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(
			`Bearer error="invalid_token", resource_metadata="%s/.well-known/oauth-protected-resource"`,
			s.baseURL,
		))
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Store Google token and pre-built HTTP client in request context.
	ctx := ContextWithGoogleToken(r.Context(), session.GoogleToken)
	ctx = ContextWithHTTPClient(ctx, s.googleOAuth.HTTPClientForToken(ctx, session.GoogleToken))
	s.mcpServer.ServeHTTP(w, r.WithContext(ctx))
}

func (s *Server) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"resource":              s.baseURL,
		"authorization_servers": []string{s.baseURL},
	}); err != nil {
		log.Printf("failed to write metadata response: %v", err)
	}
}

func (s *Server) handleAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                s.baseURL,
		"authorization_endpoint":                s.baseURL + "/authorize",
		"token_endpoint":                        s.baseURL + "/token",
		"revocation_endpoint":                   s.baseURL + "/revoke",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	}); err != nil {
		log.Printf("failed to write auth server metadata: %v", err)
	}
}
