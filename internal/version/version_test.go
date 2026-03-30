package version_test

import (
	"strings"
	"testing"

	"github.com/farhaan/llmeval/internal/version"
)

func TestGetVersion_DefaultIsDev(t *testing.T) {
	// In test binaries debug.ReadBuildInfo may return "(devel)" or no tag.
	// Either way GetVersion must return a non-empty string.
	v := version.GetVersion()
	if v == "" {
		t.Error("GetVersion returned empty string")
	}
}

func TestGetVersion_LdflagsOverride(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = "1.2.3"
	if got := version.GetVersion(); got != "1.2.3" {
		t.Errorf("GetVersion = %q, want %q", got, "1.2.3")
	}
}

func TestGetVersion_DevFallback(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = "dev"
	// Must return either "dev", "dev-<hash>", or a semver from build info.
	v := version.GetVersion()
	if v == "" {
		t.Error("GetVersion returned empty string with default Version")
	}
	// If it resolved a real build tag it should not contain spaces.
	if strings.Contains(v, " ") {
		t.Errorf("GetVersion returned string with spaces: %q", v)
	}
}

func TestGetVersion_EmptyStringTreatedAsDev(t *testing.T) {
	orig := version.Version
	t.Cleanup(func() { version.Version = orig })

	version.Version = ""
	v := version.GetVersion()
	if v == "" {
		t.Error("GetVersion returned empty string when Version=''")
	}
}
