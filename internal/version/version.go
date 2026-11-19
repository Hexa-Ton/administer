// Package version is the single source of truth for the release version.
package version

// Version is the semantic version, without a leading "v". Release builds
// override it at link time with -ldflags "-X <module>/internal/version.Version=...".
var Version = "1.0.0"
