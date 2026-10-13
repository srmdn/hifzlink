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
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/srmdn/hifzlink/internal/db"
)

const (
	googleAuthorizeURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL      = "https://oauth2.googleapis.com/token"
	cookieOAuthState    = "_oauth_state"
	cookieOAuthVerifier = "_oauth_cv"
	cookieOAuthNonce    = "_oauth_nonce"
	cookieSession       = "_session"
	oauthCookieTTL      = 10 * time.Minute
	sessionTTL          = 30 * 24 * time.Hour
)

var googleHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func generateRandomBytes(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (s *server) googleConfigured() bool {
	return s.googleClientID != "" && s.googleClientSecret != "" && s.googleRedirectURI != ""
}

func (s *server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.googleConfigured() {
		http.Error(w, "Google sign-in is not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := generateRandomBytes(32)
	if err != nil {
		internalError(w, r, err)
		return
	}
	verifier, err := generateRandomBytes(32)
	if err != nil {
		internalError(w, r, err)
		return
	}
	nonce, err := generateRandomBytes(32)
	if err != nil {
		internalError(w, r, err)
		return
	}
	setShortCookie(w, r, cookieOAuthState, state)
	setShortCookie(w, r, cookieOAuthVerifier, verifier)
	setShortCookie(w, r, cookieOAuthNonce, nonce)

	params := url.Values{
		"client_id":             {s.googleClientID},
		"redirect_uri":          {s.googleRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
		"nonce":                 {nonce},
		"prompt":                {"select_account"},
	}
	http.Redirect(w, r, googleAuthorizeURL+"?"+params.Encode(), http.StatusFound)
}

func (s *server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Query().Get("error") != "" {
		clearCookie(w, r, cookieOAuthState)
		clearCookie(w, r, cookieOAuthVerifier)
		clearCookie(w, r, cookieOAuthNonce)
		http.Redirect(w, r, "/?auth_error=denied", http.StatusSeeOther)
		return
	}

	stateCookie, err := r.Cookie(cookieOAuthState)
	if err != nil || stateCookie.Value == "" || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(stateCookie.Value)) != 1 {
		http.Error(w, "invalid sign-in state", http.StatusBadRequest)
		return
	}
	verifierCookie, err := r.Cookie(cookieOAuthVerifier)
	if err != nil || verifierCookie.Value == "" {
		http.Error(w, "missing code verifier", http.StatusBadRequest)
		return
	}
	nonceCookie, err := r.Cookie(cookieOAuthNonce)
	if err != nil || nonceCookie.Value == "" {
		http.Error(w, "missing sign-in nonce", http.StatusBadRequest)
		return
	}
	clearCookie(w, r, cookieOAuthState)
	clearCookie(w, r, cookieOAuthVerifier)
	clearCookie(w, r, cookieOAuthNonce)

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}
	tokens, err := s.exchangeGoogleCode(r, code, verifierCookie.Value)
	if err != nil {
		internalError(w, r, fmt.Errorf("Google token exchange: %w", err))
		return
	}
	identity, err := verifyGoogleIDToken(r.Context(), tokens.IDToken, s.googleClientID, nonceCookie.Value, googleHTTPClient)
	if err != nil {
		internalError(w, r, fmt.Errorf("verify Google ID token: %w", err))
		return
	}

	sessionID, err := generateRandomBytes(32)
	if err != nil {
		internalError(w, r, err)
		return
	}
	user := db.User{ID: identity.Sub, Email: strings.TrimSpace(identity.Email), Name: strings.TrimSpace(identity.Name)}
	sess := db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Provider:  "google",
		Email:     user.Email,
		Name:      user.Name,
		ExpiresAt: time.Now().Add(sessionTTL).Unix(),
	}
	if err := s.db.CreateGoogleLogin(user, sess); err != nil {
		internalError(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, withLang("/dashboard", pageLang(r)), http.StatusSeeOther)
}

func (s *server) exchangeGoogleCode(r *http.Request, code, verifier string) (*googleTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.googleRedirectURI},
		"client_id":     {s.googleClientID},
		"client_secret": {s.googleClientSecret},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}
	var tokens googleTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokens); err != nil {
		return nil, err
	}
	if tokens.IDToken == "" {
		return nil, fmt.Errorf("token endpoint returned no ID token")
	}
	return &tokens, nil
}

type googleTokenResponse struct {
	IDToken string `json:"id_token"`
}

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
	if sess, ok := s.currentSession(r); ok {
		if !s.validateCSRFToken(r, s.csrfTokenFor(sess.ID)) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		_ = s.db.DeleteSession(sess.ID)
	}
	clearCookie(w, r, cookieSession)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) currentSession(r *http.Request) (*db.Session, bool) {
	c, err := r.Cookie(cookieSession)
	if err != nil || c.Value == "" {
		return nil, false
	}
	sess, err := s.db.SessionByID(c.Value)
	if err != nil || sess.Provider != "google" || time.Now().Unix() > sess.ExpiresAt {
		return nil, false
	}
	return &sess, true
}

func setShortCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(oauthCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func secureCookieFor(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return false
	}
	return true
}

func (s *server) csrfTokenFor(id string) string {
	mac := hmac.New(sha256.New, s.csrfSecret)
	mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// validateCSRFToken compares the submitted token in constant time.
func (s *server) validateCSRFToken(r *http.Request, expected string) bool {
	got := r.FormValue("csrf_token")
	return got != "" && expected != "" && subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
