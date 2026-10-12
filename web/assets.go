package web

import "embed"

// Files contains the versioned templates and browser assets used by releases.
// Persistent state and private configuration remain external to this bundle.
//
//go:embed static/* templates/*.html
var Files embed.FS
