package main

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"path"

	webassets "github.com/srmdn/hifzlink/web"
)

const (
	assetVersionParam = "v"
	// Fingerprinted assets are safe to cache forever because a content change
	// produces a new URL.
	assetImmutableCache = "public, max-age=31536000, immutable"
	// Unversioned requests (direct links, crawlers) stay revalidatable.
	assetDefaultCache = "public, max-age=300"
)

// Release templates and browser assets travel with the executable. Persistent
// state and private configuration remain outside this embedded source bundle.
type assetVersions map[string]string

func embeddedStaticFiles() (fs.FS, error) {
	return fs.Sub(webassets.Files, "static")
}

func newAssetVersions(files fs.FS) (*assetVersions, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	versions := assetVersions{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		versions[entry.Name()] = hex.EncodeToString(sum[:])[:8]
	}
	return &versions, nil
}

// URL returns the versioned URL for a static asset, or the plain URL when the
// asset is unknown.
func (v *assetVersions) URL(name string) string {
	version := (*v)[path.Base(name)]
	if version == "" {
		return "/static/" + name
	}
	return "/static/" + name + "?" + assetVersionParam + "=" + version
}
func assetFuncs(versions *assetVersions) template.FuncMap {
	return template.FuncMap{
		"asset": versions.URL,
	}
}

// staticHandler serves embedded files and lets fingerprinted requests be cached
// permanently while leaving unversioned requests revalidatable.
func staticHandler(files fs.FS) http.Handler {
	server := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get(assetVersionParam) != "" {
			w.Header().Set("Cache-Control", assetImmutableCache)
		} else {
			w.Header().Set("Cache-Control", assetDefaultCache)
		}
		server.ServeHTTP(w, r)
	})
}
