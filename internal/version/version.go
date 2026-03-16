package version

// Set at build time via ldflags:
//
//	-ldflags "-X github.com/bnema/sekeve/internal/version.Version=v0.1.0"
var Version = "dev"
