package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
	authCodeTTL     = 5 * time.Minute
)

var validTokenFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Session struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	GoogleToken  *oauth2.Token `json:"google_token"`
	ExpiresAt    time.Time     `json:"expires_at"`
	CreatedAt    time.Time     `json:"created_at"`
}

// TokenResponse is the JSON returned by the /token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

const authRequestTTL = 10 * time.Minute

// AuthRequest tracks a pending MCP client authorization request
// while the user is redirected to Google for consent.
type AuthRequest struct {
	ClientRedirectURI   string
	ClientState         string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           time.Time
}

// AuthCode is a short-lived code returned to the MCP client
// after Google OAuth completes. Exchanged for an access token via POST /token.
type AuthCode struct {
	Code          string
	GoogleToken   *oauth2.Token
	CodeChallenge string
	RedirectURI   string
	ExpiresAt     time.Time
}

type SessionStore struct {
	dir          string
	mu           sync.RWMutex
	authRequests sync.Map // server_state -> *AuthRequest
	authCodes    sync.Map // code -> *AuthCode
	refreshIndex map[string]string // refreshToken -> accessToken
}

func NewSessionStore(dir string) (*SessionStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	store := &SessionStore{
		dir:          dir,
		refreshIndex: make(map[string]string),
	}

	// Build refresh token index from existing session files.
	if err := store.rebuildRefreshIndex(); err != nil {
		return nil, fmt.Errorf("rebuild refresh index: %w", err)
	}

	return store, nil
}

// rebuildRefreshIndex scans session files and populates the refresh token index.
func (s *SessionStore) rebuildRefreshIndex() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		accessToken := entry.Name()[:len(entry.Name())-5]
		session, err := s.readSessionFile(accessToken)
		if err != nil {
			continue
		}
		s.refreshIndex[session.RefreshToken] = accessToken
	}
	return nil
}

// StoreAuthRequest saves a pending auth request keyed by server-generated state.
func (s *SessionStore) StoreAuthRequest(serverState string, req *AuthRequest) {
	req.CreatedAt = time.Now()
	s.authRequests.Store(serverState, req)
}

// GetAuthRequest retrieves and removes a pending auth request.
func (s *SessionStore) GetAuthRequest(serverState string) (*AuthRequest, bool) {
	val, ok := s.authRequests.LoadAndDelete(serverState)
	if !ok {
		return nil, false
	}
	req := val.(*AuthRequest)
	if time.Now().After(req.CreatedAt.Add(authRequestTTL)) {
		return nil, false
	}
	return req, true
}

// StartCleanup runs a background goroutine that periodically purges
// expired auth requests and auth codes from memory.
func (s *SessionStore) StartCleanup(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				now := time.Now()
				s.authRequests.Range(func(key, val any) bool {
					if req, ok := val.(*AuthRequest); ok {
						if now.After(req.CreatedAt.Add(authRequestTTL)) {
							s.authRequests.Delete(key)
						}
					}
					return true
				})
				s.authCodes.Range(func(key, val any) bool {
					if ac, ok := val.(*AuthCode); ok {
						if now.After(ac.ExpiresAt) {
							s.authCodes.Delete(key)
						}
					}
					return true
				})
			}
		}
	}()
}

// StoreAuthCode saves a short-lived auth code.
func (s *SessionStore) StoreAuthCode(code *AuthCode) {
	s.authCodes.Store(code.Code, code)
}

// ExchangeAuthCode retrieves, validates, and removes an auth code.
// Returns the associated Google token if PKCE and redirect_uri verification pass.
func (s *SessionStore) ExchangeAuthCode(code string, codeVerifier string, redirectURI string) (*oauth2.Token, error) {
	val, ok := s.authCodes.LoadAndDelete(code)
	if !ok {
		return nil, fmt.Errorf("invalid or expired authorization code")
	}

	ac := val.(*AuthCode)

	if time.Now().After(ac.ExpiresAt) {
		return nil, fmt.Errorf("authorization code expired")
	}

	if redirectURI != ac.RedirectURI {
		return nil, fmt.Errorf("redirect_uri mismatch")
	}

	if !verifyPKCE(codeVerifier, ac.CodeChallenge) {
		return nil, fmt.Errorf("PKCE verification failed")
	}

	return ac.GoogleToken, nil
}

// CreateSession persists a new session with access + refresh tokens and returns the token response.
func (s *SessionStore) CreateSession(googleToken *oauth2.Token) (*TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accessToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now()
	session := &Session{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		GoogleToken:  googleToken,
		ExpiresAt:    now.Add(accessTokenTTL),
		CreatedAt:    now,
	}

	if err := s.writeSessionFile(accessToken, session); err != nil {
		return nil, err
	}

	s.refreshIndex[refreshToken] = accessToken

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
	}, nil
}

// GetSession loads and validates a session by access token.
// Returns an error if the token is invalid, expired, or not found.
func (s *SessionStore) GetSession(accessToken string) (*Session, error) {
	if !validTokenFormat.MatchString(accessToken) {
		return nil, fmt.Errorf("session not found")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, err := s.readSessionFile(accessToken)
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("access token expired")
	}

	return session, nil
}

// RefreshSession rotates tokens: deletes old session, creates new one with the same Google token.
// The old refresh token is invalidated (rotation). Enforces refreshTokenTTL.
func (s *SessionStore) RefreshSession(refreshToken string) (*TokenResponse, error) {
	if !validTokenFormat.MatchString(refreshToken) {
		return nil, fmt.Errorf("invalid refresh token")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// O(1) lookup via refresh index.
	oldAccessToken, ok := s.refreshIndex[refreshToken]
	if !ok {
		return nil, fmt.Errorf("invalid refresh token")
	}

	session, err := s.readSessionFile(oldAccessToken)
	if err != nil {
		delete(s.refreshIndex, refreshToken)
		return nil, fmt.Errorf("invalid refresh token")
	}

	if session.RefreshToken != refreshToken {
		delete(s.refreshIndex, refreshToken)
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Enforce refresh token TTL.
	if time.Now().After(session.CreatedAt.Add(refreshTokenTTL)) {
		delete(s.refreshIndex, refreshToken)
		_ = os.Remove(filepath.Join(s.dir, oldAccessToken+".json"))
		return nil, fmt.Errorf("refresh token expired")
	}

	// Delete the old session file and index entry.
	_ = os.Remove(filepath.Join(s.dir, oldAccessToken+".json"))
	delete(s.refreshIndex, refreshToken)

	// Generate new tokens.
	newAccessToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	newRefreshToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now()
	newSession := &Session{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		GoogleToken:  session.GoogleToken,
		ExpiresAt:    now.Add(accessTokenTTL),
		CreatedAt:    now,
	}

	if err := s.writeSessionFile(newAccessToken, newSession); err != nil {
		return nil, err
	}

	s.refreshIndex[newRefreshToken] = newAccessToken

	return &TokenResponse{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		RefreshToken: newRefreshToken,
	}, nil
}

// RevokeSession removes a session by access token.
func (s *SessionStore) RevokeSession(accessToken string) {
	if !validTokenFormat.MatchString(accessToken) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.readSessionFile(accessToken)
	if err == nil {
		delete(s.refreshIndex, session.RefreshToken)
	}

	path := filepath.Join(s.dir, accessToken+".json")
	_ = os.Remove(path)
}

// writeSessionFile atomically writes a session to disk using tmp + rename.
// Must be called with s.mu held.
func (s *SessionStore) writeSessionFile(accessToken string, session *Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(s.dir, accessToken+".json")
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write session tmp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename session: %w", err)
	}

	return nil
}

// readSessionFile reads a session from disk.
// Must be called with s.mu held (read or write).
func (s *SessionStore) readSessionFile(accessToken string) (*Session, error) {
	path := filepath.Join(s.dir, accessToken+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func verifyPKCE(codeVerifier, codeChallenge string) bool {
	if codeChallenge == "" || codeVerifier == "" {
		return false
	}
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == codeChallenge
}
