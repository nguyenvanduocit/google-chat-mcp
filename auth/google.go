package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOAuth struct {
	config *oauth2.Config
	store  *SessionStore
}

// HTTPClientForToken creates an authenticated HTTP client using the cached OAuth config.
func (g *GoogleOAuth) HTTPClientForToken(ctx context.Context, token *oauth2.Token) *http.Client {
	return g.config.Client(ctx, token)
}

func NewGoogleOAuth(credentialsFile string, scopes []string, store *SessionStore, baseURL string) (*GoogleOAuth, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	config.RedirectURL = baseURL + "/oauth/callback"

	return &GoogleOAuth{
		config: config,
		store:  store,
	}, nil
}

// HandleAuthorize receives the MCP client's authorization request
// and redirects the user to Google for consent.
//
// Expected query params from MCP client:
// - response_type=code
// - redirect_uri (MCP client's callback, must be loopback)
// - state (MCP client's CSRF token)
// - code_challenge (PKCE S256, required)
// - code_challenge_method=S256
func (g *GoogleOAuth) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	clientRedirectURI := q.Get("redirect_uri")
	clientState := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")

	if clientRedirectURI == "" {
		http.Error(w, "redirect_uri is required", http.StatusBadRequest)
		return
	}

	if !isAllowedRedirectURI(clientRedirectURI) {
		http.Error(w, "redirect_uri must be a loopback address", http.StatusBadRequest)
		return
	}

	if codeChallenge == "" || codeChallengeMethod != "S256" {
		http.Error(w, "code_challenge with method S256 is required", http.StatusBadRequest)
		return
	}

	serverState, err := generateRandomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	g.store.StoreAuthRequest(serverState, &AuthRequest{
		ClientRedirectURI:   clientRedirectURI,
		ClientState:         clientState,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	})

	googleAuthURL := g.config.AuthCodeURL(serverState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, googleAuthURL, http.StatusFound)
}

// HandleCallback processes Google's OAuth callback,
// generates our own auth code, and redirects back to the MCP client.
func (g *GoogleOAuth) HandleCallback(w http.ResponseWriter, r *http.Request) {
	serverState := r.URL.Query().Get("state")
	googleCode := r.URL.Query().Get("code")
	googleError := r.URL.Query().Get("error")

	if googleError != "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, "Google OAuth error: %s", googleError)
		return
	}

	if googleCode == "" {
		http.Error(w, "missing authorization code from Google", http.StatusBadRequest)
		return
	}

	authReq, ok := g.store.GetAuthRequest(serverState)
	if !ok {
		http.Error(w, "invalid or expired state parameter", http.StatusBadRequest)
		return
	}

	// Generate our auth code BEFORE exchanging with Google,
	// so we don't leak a Google token if code generation fails.
	ourCode, err := generateRandomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	googleToken, err := g.config.Exchange(r.Context(), googleCode)
	if err != nil {
		log.Printf("Google token exchange error: %v", err)
		http.Error(w, "failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	g.store.StoreAuthCode(&AuthCode{
		Code:          ourCode,
		GoogleToken:   googleToken,
		CodeChallenge: authReq.CodeChallenge,
		RedirectURI:   authReq.ClientRedirectURI,
		ExpiresAt:     time.Now().Add(authCodeTTL),
	})

	params := url.Values{}
	params.Set("code", ourCode)
	if authReq.ClientState != "" {
		params.Set("state", authReq.ClientState)
	}
	redirectURL := authReq.ClientRedirectURI + "?" + params.Encode()

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleToken handles both authorization_code and refresh_token grant types.
func (g *GoogleOAuth) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		writeTokenError(w, "invalid_request", "failed to parse form")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		g.handleAuthCodeExchange(w, r)
	case "refresh_token":
		g.handleRefreshToken(w, r)
	default:
		writeTokenError(w, "unsupported_grant_type", "supported: authorization_code, refresh_token")
	}
}

func (g *GoogleOAuth) handleAuthCodeExchange(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")
	redirectURI := r.FormValue("redirect_uri")

	if code == "" {
		writeTokenError(w, "invalid_request", "code is required")
		return
	}

	googleToken, err := g.store.ExchangeAuthCode(code, codeVerifier, redirectURI)
	if err != nil {
		writeTokenError(w, "invalid_grant", err.Error())
		return
	}

	tokenResp, err := g.store.CreateSession(googleToken)
	if err != nil {
		writeTokenError(w, "server_error", "failed to create session")
		return
	}

	writeJSON(w, tokenResp)
}

func (g *GoogleOAuth) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	if refreshToken == "" {
		writeTokenError(w, "invalid_request", "refresh_token is required")
		return
	}

	tokenResp, err := g.store.RefreshSession(refreshToken)
	if err != nil {
		writeTokenError(w, "invalid_grant", err.Error())
		return
	}

	writeJSON(w, tokenResp)
}

// HandleRevoke revokes an access token.
func (g *GoogleOAuth) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token != "" {
		g.store.RevokeSession(token)
	}

	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

func writeTokenError(w http.ResponseWriter, errorCode string, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	}); err != nil {
		log.Printf("failed to write error response: %v", err)
	}
}

// isAllowedRedirectURI ensures only loopback addresses are accepted.
func isAllowedRedirectURI(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func generateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
