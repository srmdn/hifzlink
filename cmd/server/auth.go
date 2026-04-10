package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/srmdn/hifzlink/internal/db"
)

const (
	cookieOAuthState    = "_oauth_state"
	cookieOAuthVerifier = "_oauth_cv"
	cookieOAuthNonce    = "_oauth_nonce"
	cookieSession       = "_session"
	oauthCookieTTL      = 10 * time.Minute
	sessionTTL          = 30 * 24 * time.Hour
)

// generateRandomBytes returns n cryptographically random bytes as base64url (no padding).
func generateRandomBytes(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge computes the S256 code challenge from a verifier.
func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// qfConfigured returns true when all required QF OAuth2 env vars are set.
func (s *server) qfConfigured() bool {
	return s.qfClientID != "" && s.qfClientSecret != "" && s.qfAuthEndpoint != ""
}

// qfCallbackURI returns the redirect URI for this request's host, or the
// configured override if QF_REDIRECT_URI is set.
func (s *server) qfCallbackURI(r *http.Request) string {
	if s.qfRedirectURI != "" {
		return s.qfRedirectURI
	}
	return "https://" + r.Host + "/auth/callback"
}

// handleAuthLogin starts the OAuth2 authorization code + PKCE flow.
func (s *server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.qfConfigured() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state, err := generateRandomBytes(16)
	if err != nil {
		internalError(w, r, err)
		return
	}
	verifier, err := generateRandomBytes(32)
	if err != nil {
		internalError(w, r, err)
		return
	}
	nonce, err := generateRandomBytes(16)
	if err != nil {
		internalError(w, r, err)
		return
	}

	setShortCookie(w, cookieOAuthState, state)
	setShortCookie(w, cookieOAuthVerifier, verifier)
	setShortCookie(w, cookieOAuthNonce, nonce)

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {s.qfClientID},
		"redirect_uri":          {s.qfCallbackURI(r)},
		"scope":                 {"openid offline_access user collection bookmark"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, s.qfAuthEndpoint+"/oauth2/auth?"+params.Encode(), http.StatusFound)
}

// handleAuthCallback handles the redirect back from QF after user login.
func (s *server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// User denied authorization or QF returned an error.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		log.Printf("QF OAuth error: %s | description: %s | full URL: %s", errParam, errDesc, r.URL.RawQuery)
		http.Redirect(w, r, "/?auth_error=denied", http.StatusSeeOther)
		return
	}

	// Validate state to prevent CSRF.
	stateCookie, err := r.Cookie(cookieOAuthState)
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing oauth state", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	cvCookie, err := r.Cookie(cookieOAuthVerifier)
	if err != nil || cvCookie.Value == "" {
		http.Error(w, "missing code verifier", http.StatusBadRequest)
		return
	}

	// Clear PKCE/nonce cookies immediately.
	clearCookie(w, cookieOAuthState)
	clearCookie(w, cookieOAuthVerifier)
	clearCookie(w, cookieOAuthNonce)

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	tokens, err := s.exchangeCode(r, code, cvCookie.Value)
	if err != nil {
		internalError(w, r, fmt.Errorf("token exchange: %w", err))
		return
	}

	userInfo, err := s.fetchUserInfo(tokens.AccessToken)
	if err != nil {
		internalError(w, r, fmt.Errorf("userinfo: %w", err))
		return
	}

	sessionID, err := generateRandomBytes(24)
	if err != nil {
		internalError(w, r, err)
		return
	}

	sess := db.Session{
		ID:           sessionID,
		UserID:       userInfo.Sub,
		Email:        userInfo.Email,
		Name:         strings.TrimSpace(userInfo.FirstName + " " + userInfo.LastName),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    time.Now().Add(sessionTTL).Unix(), // session lives 30 days; access token is refreshed separately
	}
	if err := s.db.CreateSession(sess); err != nil {
		internalError(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	lang := pageLang(r)
	http.Redirect(w, r, withLang("/dashboard", lang), http.StatusSeeOther)
}

// handleAuthLogout clears the session cookie and deletes the session from the DB.
func (s *server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Validate CSRF only when there is an active session (logout without a session is a no-op anyway).
	if sess, ok := s.currentSession(r); ok {
		if !s.validateCSRFToken(r, s.csrfTokenFor(sess.ID)) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		_ = s.db.DeleteSession(sess.ID)
	}
	clearCookie(w, cookieSession)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// currentSession looks up the session for the current request, if any.
func (s *server) currentSession(r *http.Request) (*db.Session, bool) {
	c, err := r.Cookie(cookieSession)
	if err != nil || c.Value == "" {
		return nil, false
	}
	sess, err := s.db.SessionByID(c.Value)
	if err != nil {
		return nil, false
	}
	if time.Now().Unix() > sess.ExpiresAt {
		return nil, false
	}
	return &sess, true
}

// tokenResponse is the JSON body returned by the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// userInfoResponse is the JSON body returned by the userinfo endpoint.
type userInfoResponse struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (s *server) exchangeCode(r *http.Request, code, verifier string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.qfCallbackURI(r)},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequest(http.MethodPost, s.qfAuthEndpoint+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.qfClientID, s.qfClientSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (s *server) fetchUserInfo(accessToken string) (*userInfoResponse, error) {
	req, err := http.NewRequest(http.MethodGet, s.qfAuthEndpoint+"/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, body)
	}
	var info userInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func setShortCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(oauthCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// refreshSession exchanges the stored refresh token for a new access token,
// updates the session in the DB, and returns the new access token.
// The sess struct is updated in place on success.
func (s *server) refreshSession(sess *db.Session) (string, error) {
	if sess.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token stored for session")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {sess.RefreshToken},
	}
	req, err := http.NewRequest(http.MethodPost, s.qfAuthEndpoint+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.qfClientID, s.qfClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read refresh response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh token: HTTP %d: %s", resp.StatusCode, body)
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("decode refresh response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access token in refresh response")
	}

	newExpiry := time.Now().Add(sessionTTL).Unix()
	newRefresh := tok.RefreshToken
	if newRefresh == "" {
		newRefresh = sess.RefreshToken // keep the old one if not rotated
	}

	if err := s.db.UpdateSessionTokens(sess.ID, tok.AccessToken, newRefresh, newExpiry); err != nil {
		return "", fmt.Errorf("persist refreshed tokens: %w", err)
	}

	sess.AccessToken = tok.AccessToken
	sess.RefreshToken = newRefresh
	sess.ExpiresAt = newExpiry
	log.Printf("auth: refreshed session %s", sess.ID[:8])
	return tok.AccessToken, nil
}

// csrfTokenFor derives a CSRF token from a session ID (or admin token)
// using HMAC-SHA256 with the server's CSRF secret.
func (s *server) csrfTokenFor(id string) string {
	mac := hmac.New(sha256.New, s.csrfSecret)
	mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// validateCSRFToken returns true if the csrf_token form field matches the expected value.
// Must be called after ParseForm.
func (s *server) validateCSRFToken(r *http.Request, expected string) bool {
	got := r.FormValue("csrf_token")
	if got == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
