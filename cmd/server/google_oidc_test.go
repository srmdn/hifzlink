package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type googleTestClaims struct {
	Issuer        string          `json:"iss"`
	Audience      json.RawMessage `json:"aud"`
	AuthorizedBy  string          `json:"azp,omitempty"`
	Subject       string          `json:"sub"`
	Email         string          `json:"email"`
	EmailVerified bool            `json:"email_verified"`
	Name          string          `json:"name"`
	ExpiresAt     int64           `json:"exp"`
	IssuedAt      int64           `json:"iat"`
	Nonce         string          `json:"nonce"`
}

func makeGoogleTestClaims(nonce string) googleTestClaims {
	audience, _ := json.Marshal("test-client-id")
	now := time.Now().Unix()
	return googleTestClaims{
		Issuer:        googleIssuerURL,
		Audience:      audience,
		Subject:       "google-sub",
		Email:         "reader@example.com",
		EmailVerified: true,
		Name:          "Reader",
		ExpiresAt:     now + 300,
		IssuedAt:      now,
		Nonce:         nonce,
	}
}

func signGoogleTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims googleTestClaims) string {
	t.Helper()
	header, err := json.Marshal(struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}{Algorithm: "RS256", KeyID: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func googleTestVerifier(privateKey *rsa.PrivateKey) *oidc.IDTokenVerifier {
	keySet := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&privateKey.PublicKey}}
	return oidc.NewVerifier(googleIssuerURL, keySet, &oidc.Config{
		ClientID:             "test-client-id",
		SupportedSigningAlgs: []string{"RS256"},
	})
}

func TestVerifyGoogleIDTokenValidatesIdentity(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const nonce = "nonce-for-this-browser"
	token := signGoogleTestToken(t, privateKey, makeGoogleTestClaims(nonce))
	identity, err := verifyGoogleIDTokenWithVerifier(context.Background(), token, "test-client-id", nonce, googleTestVerifier(privateKey))
	if err != nil {
		t.Fatalf("verify signed Google token: %v", err)
	}
	if identity.Sub != "google-sub" || identity.Email != "reader@example.com" || identity.Name != "Reader" {
		t.Fatalf("unexpected verified identity: %+v", identity)
	}
}

func TestVerifyGoogleIDTokenRejectsInvalidClaims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name   string
		change func(*googleTestClaims)
	}{
		{name: "wrong issuer", change: func(c *googleTestClaims) { c.Issuer = "https://attacker.example" }},
		{name: "wrong audience", change: func(c *googleTestClaims) { c.Audience, _ = json.Marshal("another-client") }},
		{name: "wrong nonce", change: func(c *googleTestClaims) { c.Nonce = "old-nonce" }},
		{name: "expired", change: func(c *googleTestClaims) { c.ExpiresAt = time.Now().Add(-time.Minute).Unix() }},
		{name: "unverified email", change: func(c *googleTestClaims) { c.EmailVerified = false }},
		{name: "future issued at", change: func(c *googleTestClaims) { c.IssuedAt = time.Now().Add(2 * time.Minute).Unix() }},
		{name: "wrong authorized party", change: func(c *googleTestClaims) {
			c.Audience, _ = json.Marshal([]string{"test-client-id", "other-client"})
			c.AuthorizedBy = "other-client"
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			claims := makeGoogleTestClaims("expected-nonce")
			check.change(&claims)
			token := signGoogleTestToken(t, privateKey, claims)
			_, err := verifyGoogleIDTokenWithVerifier(context.Background(), token, "test-client-id", "expected-nonce", googleTestVerifier(privateKey))
			if err == nil {
				t.Fatal("expected invalid ID token claims to be rejected")
			}
		})
	}
}

func TestVerifyGoogleIDTokenRejectsInvalidSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	token := signGoogleTestToken(t, privateKey, makeGoogleTestClaims("expected-nonce"))
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"https://accounts.google.com"}`))
	tampered := strings.Join(parts, ".")
	_, err = verifyGoogleIDTokenWithVerifier(context.Background(), tampered, "test-client-id", "expected-nonce", googleTestVerifier(privateKey))
	if err == nil {
		t.Fatal("tampered ID token signature was accepted")
	}
}

func TestVerifyGoogleIDTokenRejectsOversizedToken(t *testing.T) {
	_, err := verifyGoogleIDToken(context.Background(), strings.Repeat("x", (16<<10)+1), "test-client-id", "nonce", nil)
	if err == nil {
		t.Fatal("oversized ID token was accepted")
	}
}
