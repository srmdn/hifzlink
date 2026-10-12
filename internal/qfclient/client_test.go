package qfclient

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTokenDoesNotForwardCredentialsAcrossRedirect(t *testing.T) {
	var requests int
	client := New("client-id", "client-secret", "https://oauth.example.test", "https://apis-prelive.quran.foundation")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if got := r.Header.Get("Authorization"); got == "" {
			t.Error("token request did not use client authentication")
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://redirect.example.test/token"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})
	if _, err := client.token(); err == nil {
		t.Fatal("token request unexpectedly followed redirect")
	}
	if requests != 1 {
		t.Fatalf("transport received %d requests; want 1", requests)
	}
}
