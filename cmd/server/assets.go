package main

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	assetVersionParam = "v"
	// Fingerprinted assets are safe to cache forever because a content change
	// produces a new URL.
	assetImmutableCache = "public, max-age=31536000, immutable"
	// Unversioned requests (direct links, crawlers) stay revalidatable.
	assetDefaultCache = "public, max-age=300"
)

// assetVersions maps a static file name to a short content hash.
type assetVersions map[string]string

// loadAssetVersions fingerprints every file in staticDir so templates can bust
// client and CDN caches by content instead of by build revision.
func loadAssetVersions(staticDir string) assetVersions {
	versions := assetVersions{}

	entries, err := os.ReadDir(staticDir)
	if err != nil {
		log.Fatalf("failed to read static dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(staticDir, name))
		if err != nil {
			log.Printf("warn: read static asset %s: %v", name, err)
			continue
		}
		sum := sha256.Sum256(data)
		versions[name] = hex.EncodeToString(sum[:])[:8]
	}

	return versions
}

// URL returns the versioned URL for a static asset, or the plain URL when the
// asset is unknown.
func (v assetVersions) URL(name string) string {
	version, ok := v[name]
	if !ok || version == "" {
		return "/static/" + name
	}
	return "/static/" + name + "?" + assetVersionParam + "=" + version
}

func assetFuncs(versions assetVersions) template.FuncMap {
	return template.FuncMap{
		"asset": versions.URL,
	}
}

// staticHandler serves dir and lets fingerprinted requests be cached
// permanently while leaving unversioned requests revalidatable.
func staticHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get(assetVersionParam) != "" {
			w.Header().Set("Cache-Control", assetImmutableCache)
		} else {
			w.Header().Set("Cache-Control", assetDefaultCache)
		}
		files.ServeHTTP(w, r)
	})
}
