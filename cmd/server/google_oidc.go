package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	googleIssuerURL = "https://accounts.google.com"
	googleJWKSURL   = "https://www.googleapis.com/oauth2/v3/certs"
)

type googleIDTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	AuthorizedBy  string `json:"azp"`
}

type googleIdentity struct {
	Sub   string
	Email string
	Name  string
}

var (
	googleOIDCVerifierMu sync.Mutex
	googleOIDCVerifiers  = make(map[string]*oidc.IDTokenVerifier)
)

func verifyGoogleIDToken(ctx context.Context, rawToken, clientID, expectedNonce string, client *http.Client) (googleIdentity, error) {
	if len(rawToken) == 0 || len(rawToken) > 16<<10 {
		return googleIdentity{}, fmt.Errorf("invalid ID token size")
	}
	if clientID == "" || expectedNonce == "" || client == nil {
		return googleIdentity{}, fmt.Errorf("Google ID token verification is not configured")
	}
	return verifyGoogleIDTokenWithVerifier(ctx, rawToken, clientID, expectedNonce, googleOIDCVerifier(clientID, client))
}

func googleOIDCVerifier(clientID string, client *http.Client) *oidc.IDTokenVerifier {
	googleOIDCVerifierMu.Lock()
	defer googleOIDCVerifierMu.Unlock()
	if verifier := googleOIDCVerifiers[clientID]; verifier != nil {
		return verifier
	}
	keySet := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), client), googleJWKSURL)
	verifier := oidc.NewVerifier(googleIssuerURL, keySet, &oidc.Config{
		ClientID:             clientID,
		SupportedSigningAlgs: []string{"RS256"},
	})
	googleOIDCVerifiers[clientID] = verifier
	return verifier
}

func verifyGoogleIDTokenWithVerifier(ctx context.Context, rawToken, clientID, expectedNonce string, verifier *oidc.IDTokenVerifier) (googleIdentity, error) {
	if verifier == nil || clientID == "" || expectedNonce == "" {
		return googleIdentity{}, fmt.Errorf("Google ID token verification is not configured")
	}
	token, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return googleIdentity{}, fmt.Errorf("verify signed ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(expectedNonce)) != 1 {
		return googleIdentity{}, fmt.Errorf("ID token nonce does not match this sign-in")
	}
	if token.IssuedAt.IsZero() || token.IssuedAt.After(time.Now().Add(time.Minute)) || !token.IssuedAt.Before(token.Expiry) {
		return googleIdentity{}, fmt.Errorf("ID token has invalid timestamps")
	}
	var claims googleIDTokenClaims
	if err := token.Claims(&claims); err != nil {
		return googleIdentity{}, fmt.Errorf("decode ID token claims: %w", err)
	}
	if claims.AuthorizedBy != "" && claims.AuthorizedBy != clientID {
		return googleIdentity{}, fmt.Errorf("ID token authorized party does not match this client")
	}
	if len(token.Audience) > 1 && (claims.AuthorizedBy != clientID || !containsString(token.Audience, claims.AuthorizedBy)) {
		return googleIdentity{}, fmt.Errorf("ID token has no valid authorized party")
	}
	email := strings.TrimSpace(claims.Email)
	if token.Subject == "" || len(token.Subject) > 255 || email == "" || !claims.EmailVerified {
		return googleIdentity{}, fmt.Errorf("ID token has no verified account identity")
	}
	return googleIdentity{Sub: token.Subject, Email: email, Name: strings.TrimSpace(claims.Name)}, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
