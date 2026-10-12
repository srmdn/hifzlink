package main

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	assetVersionParam = "v"
	// Fingerprinted assets are safe to cache forever because a content change
	// produces a new URL.
	assetImmutableCache = "public, max-age=31536000, immutable"
	// Unversioned requests (direct links, crawlers) stay revalidatable.
	assetDefaultCache = "public, max-age=300"
)

type assetFingerprint struct {
	version string
	modTime time.Time
	size    int64
}

// assetVersions resolves a static file name to a short content hash. The hash
// is recomputed only when the file's modification time or size changed, so a
// running server picks up edited assets without a restart while fingerprinted
// URLs stay valid for immutable caching.
type assetVersions struct {
	dir string

	mu      sync.RWMutex
	entries map[string]assetFingerprint
}

func newAssetVersions(dir string) *assetVersions {
	if _, err := os.ReadDir(dir); err != nil {
		log.Fatalf("failed to read static dir: %v", err)
	}
	return &assetVersions{
		dir:     dir,
		entries: make(map[string]assetFingerprint),
	}
}

// URL returns the versioned URL for a static asset, or the plain URL when the
// asset is unknown.
func (v *assetVersions) URL(name string) string {
	version := v.version(name)
	if version == "" {
		return "/static/" + name
	}
	return "/static/" + name + "?" + assetVersionParam + "=" + version
}

func (v *assetVersions) version(name string) string {
	path := filepath.Join(v.dir, filepath.Base(name))

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}

	v.mu.RLock()
	cached, ok := v.entries[filepath.Base(name)]
	v.mu.RUnlock()
	if ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.version
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("warn: read static asset %s: %v", name, err)
		return ""
	}
	sum := sha256.Sum256(data)
	version := hex.EncodeToString(sum[:])[:8]

	v.mu.Lock()
	v.entries[filepath.Base(name)] = assetFingerprint{
		version: version,
		modTime: info.ModTime(),
		size:    info.Size(),
	}
	v.mu.Unlock()

	return version
}

func assetFuncs(versions *assetVersions) template.FuncMap {
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
