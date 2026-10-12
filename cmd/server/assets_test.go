package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAssetsAreFingerprintedAndServed(t *testing.T) {
	staticFiles, err := embeddedStaticFiles()
	if err != nil {
		t.Fatal(err)
	}
	versions, err := newAssetVersions(staticFiles)
	if err != nil {
		t.Fatal(err)
	}
	assetURL := versions.URL("home.css")
	if !strings.HasPrefix(assetURL, "/static/home.css?v=") {
		t.Fatalf("unexpected embedded asset URL: %s", assetURL)
	}

	request := httptest.NewRequest(http.MethodGet, assetURL, nil)
	response := httptest.NewRecorder()
	http.StripPrefix("/static/", staticHandler(staticFiles)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("asset request returned %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), ".home-main") {
		t.Fatal("served home.css does not contain the embedded learning styles")
	}
	if got := response.Header().Get("Cache-Control"); got != assetImmutableCache {
		t.Fatalf("fingerprinted asset cache policy = %q, want %q", got, assetImmutableCache)
	}
}

func TestUnversionedEmbeddedAssetRemainsRevalidatable(t *testing.T) {
	staticFiles, err := embeddedStaticFiles()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/static/home.css", nil)
	response := httptest.NewRecorder()
	http.StripPrefix("/static/", staticHandler(staticFiles)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("asset request returned %d", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != assetDefaultCache {
		t.Fatalf("unversioned asset cache policy = %q, want %q", got, assetDefaultCache)
	}
}
