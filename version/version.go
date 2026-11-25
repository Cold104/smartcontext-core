// Package version is the single source for the Go binary version string.
package version

// Version is the SmartContext Go MCP binary (v1.3 train).
// Default matches the repo tip; release.yml stamps via:
//
//	-ldflags="-X github.com/Cold104/smartcontext-core/version.Version=<semver>"
var Version = "1.3.0"
