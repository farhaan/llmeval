package version

import (
	"runtime/debug"
	"strings"
)

// Version is overridable at build time:
//
//	go build -ldflags "-X github.com/farhaan/llmeval/internal/version.Version=0.1.0"
//
// When installed via "go install ...@latest" without ldflags, GetVersion falls
// back to debug.ReadBuildInfo which carries the git tag from the module proxy.
var Version = "dev"

// GetVersion resolves the version string in priority order so release builds, go-install
// builds, and local dev builds all surface a meaningful identifier without manual maintenance.
func GetVersion() string {
	if Version != "dev" && Version != "" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	// go install from a tagged release: Main.Version is e.g. "v0.1.0"
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}

	// Untagged local build: use short commit hash so versions are distinguishable.
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return "dev-" + s.Value[:7]
		}
	}

	return "dev"
}
