// Package version defines globals containing version metadata set at build-time
package version

var (
	// Version is a SemVer 2.0 formatted version string
	Version string

	// Build is an opaque string identifying the automated build job
	Build string
)
