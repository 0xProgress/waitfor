// Package version holds build-time version metadata, injected via -ldflags.
package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line human-readable version string.
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
