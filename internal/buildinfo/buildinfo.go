// Package buildinfo exposes the version, commit and build date injected at
// link time via -ldflags "-X specforge/internal/buildinfo.Version=...".
// Defaults identify a local development build.
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line human-readable description of the build.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
