package main

// Version is Porter's release version. Bumped per the release process
// in versions.md — currently tracking the first public preview,
// v0.1.0-beta ("Core Runtime & First Deployment").
//
// Overridden at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X main.Version=v0.1.0-beta" ./...
//
// The .github/workflows/release.yml workflow sets this automatically
// from the pushed git tag, so this string only matters for local/dev
// builds run without that flag.
var Version = "v0.1.0-beta-dev"
