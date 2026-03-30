package harden_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farhaan/llmeval/internal/harden"
)

func writeManifest(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validManifestYAML = `
prompts:
  - path: prompts/assistant.md
    name: assistant
    description: A helpful assistant prompt
    risk_profile: direct_injection
    expected_output: Polite helpful answer
`

func TestLoadManifest_HappyPath(t *testing.T) {
	path := writeManifest(t, validManifestYAML)
	entries, err := harden.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Path != "prompts/assistant.md" {
		t.Errorf("Path = %q", e.Path)
	}
	if e.Name != "assistant" {
		t.Errorf("Name = %q", e.Name)
	}
	if e.RiskProfile != "direct_injection" {
		t.Errorf("RiskProfile = %q", e.RiskProfile)
	}
}

func TestLoadManifest_MultiplePrompts(t *testing.T) {
	yaml := `
prompts:
  - path: p1.md
    name: first
  - path: p2.md
    name: second
  - path: p3.md
    name: third
`
	path := writeManifest(t, yaml)
	entries, err := harden.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

func TestLoadManifest_EmptyFile(t *testing.T) {
	path := writeManifest(t, "")
	entries, err := harden.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty manifest, got %d", len(entries))
	}
}

func TestLoadManifest_FileNotFound(t *testing.T) {
	_, err := harden.LoadManifest("/nonexistent/manifest.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	path := writeManifest(t, "prompts: [this is: invalid: yaml: :")
	_, err := harden.LoadManifest(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestFillFixtures_Basic(t *testing.T) {
	tmpl := "Hello {{name}}, you are {{role}}."
	fixtures := map[string]string{
		"name": "Alice",
		"role": "assistant",
	}
	got := harden.FillFixtures(tmpl, fixtures)
	want := "Hello Alice, you are assistant."
	if got != want {
		t.Errorf("FillFixtures = %q, want %q", got, want)
	}
}

func TestFillFixtures_NoPlaceholders(t *testing.T) {
	tmpl := "No placeholders here."
	got := harden.FillFixtures(tmpl, map[string]string{"key": "val"})
	if got != tmpl {
		t.Errorf("FillFixtures changed template with no matching placeholder: %q", got)
	}
}

func TestFillFixtures_EmptyFixtures(t *testing.T) {
	tmpl := "{{name}} is {{role}}"
	got := harden.FillFixtures(tmpl, nil)
	if got != tmpl {
		t.Errorf("FillFixtures with nil fixtures changed template: %q", got)
	}
}

func TestFillFixtures_ReplacesAllOccurrences(t *testing.T) {
	tmpl := "{{x}} and {{x}}"
	got := harden.FillFixtures(tmpl, map[string]string{"x": "hello"})
	want := "hello and hello"
	if got != want {
		t.Errorf("FillFixtures = %q, want %q", got, want)
	}
}
