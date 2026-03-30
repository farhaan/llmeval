package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farhaan/llmeval/internal/config"
)

// ── EnvVar derivation ─────────────────────────────────────────────────────────

func TestEnvVar_DerivedFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"groq", "GROQ_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"myservice", "MYSERVICE_API_KEY"},
		{"deepinfra", "DEEPINFRA_API_KEY"},
	}
	for _, tc := range cases {
		pc := config.ProviderConfig{Type: config.ProviderOpenAICompat}
		if got := pc.EnvVar(tc.name); got != tc.want {
			t.Errorf("EnvVar(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestEnvVar_ExplicitOverride(t *testing.T) {
	pc := config.ProviderConfig{
		Type:      config.ProviderOpenAICompat,
		APIKeyEnv: "MY_CUSTOM_KEY",
	}
	if got := pc.EnvVar("groq"); got != "MY_CUSTOM_KEY" {
		t.Errorf("EnvVar with override = %q, want %q", got, "MY_CUSTOM_KEY")
	}
}

// ── Default config ────────────────────────────────────────────────────────────

func TestDefault_BuiltInProviders(t *testing.T) {
	cfg := config.Default()
	want := []string{"groq", "openai", "anthropic", "openrouter", "deepinfra"}
	for _, name := range want {
		if _, ok := cfg.Providers[name]; !ok {
			t.Errorf("default config missing provider %q", name)
		}
	}
}

func TestDefault_Sensible(t *testing.T) {
	cfg := config.Default()
	if cfg.DatasetSize <= 0 {
		t.Errorf("DatasetSize = %d, want > 0", cfg.DatasetSize)
	}
	if cfg.Concurrency <= 0 {
		t.Errorf("Concurrency = %d, want > 0", cfg.Concurrency)
	}
	if cfg.ModelConfig.MaxTokens <= 0 {
		t.Errorf("MaxTokens = %d, want > 0", cfg.ModelConfig.MaxTokens)
	}
}

// ── Load / merge ──────────────────────────────────────────────────────────────

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "llmeval*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoad_EmptyPath_ReturnsDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Providers) == 0 {
		t.Error("expected built-in providers, got none")
	}
}

func TestLoad_OverridesDatasetSize(t *testing.T) {
	path := writeYAML(t, "dataset_size: 10\n")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatasetSize != 10 {
		t.Errorf("DatasetSize = %d, want 10", cfg.DatasetSize)
	}
}

func TestLoad_AddsCustomProvider(t *testing.T) {
	path := writeYAML(t, `
providers:
  ollama:
    type: openai_compat
    base_url: http://localhost:11434/v1
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := cfg.Providers["ollama"]
	if !ok {
		t.Fatal("ollama provider not found after merge")
	}
	if p.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("ollama base_url = %q", p.BaseURL)
	}
	// Built-in providers must still be present.
	if _, ok := cfg.Providers["groq"]; !ok {
		t.Error("groq provider disappeared after merge")
	}
}

func TestLoad_AddsTasks(t *testing.T) {
	path := writeYAML(t, `
tasks:
  my-task:
    description: "test task"
    mode: pointwise
    models: [groq/llama-3.3-70b-versatile, openai/gpt-4o]
    dataset_size: 3
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	task, ok := cfg.Tasks["my-task"]
	if !ok {
		t.Fatal("my-task not found")
	}
	if task.Description != "test task" {
		t.Errorf("description = %q", task.Description)
	}
	if task.DatasetSize != 3 {
		t.Errorf("dataset_size = %d, want 3", task.DatasetSize)
	}
}

func TestLoad_MissingFile_Error(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML_Error(t *testing.T) {
	// Unclosed flow mapping is a hard YAML parse error.
	path := writeYAML(t, "providers: {unclosed\n")
	if _, err := config.Load(path); err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoad_DisableBuiltInProvider(t *testing.T) {
	path := writeYAML(t, `
providers:
  openai:
    enabled: false
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Providers["openai"]; ok {
		t.Error("openai should have been removed by enabled: false")
	}
	// Other built-in providers must be unaffected.
	if _, ok := cfg.Providers["groq"]; !ok {
		t.Error("groq should still be present")
	}
}

func TestLoad_EnabledTrueKeepsProvider(t *testing.T) {
	path := writeYAML(t, `
providers:
  groq:
    enabled: true
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Providers["groq"]; !ok {
		t.Error("groq should still be present when enabled: true")
	}
}
