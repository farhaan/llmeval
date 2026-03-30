package hasher_test

import (
	"testing"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
	"github.com/farhaan/llmeval/internal/hasher"
)

var sampleDS = dataset.Dataset{
	Task: "Explain recursion",
	Prompts: []dataset.Prompt{
		{ID: "p1", User: "What is recursion?"},
		{ID: "p2", User: "Give an example of recursion in Go."},
	},
}

var defaultCfg = config.ModelConfig{MaxTokens: 1024, Temperature: 0.7}

func TestCompute_Deterministic(t *testing.T) {
	models := []string{"groq/llama-3.3-70b-versatile", "openai/gpt-4o"}
	h1 := hasher.Compute(models, sampleDS, defaultCfg)
	h2 := hasher.Compute(models, sampleDS, defaultCfg)
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h1, h2)
	}
}

func TestCompute_ModelOrderIndependent(t *testing.T) {
	a := hasher.Compute([]string{"openai/gpt-4o", "groq/llama-3.3-70b-versatile"}, sampleDS, defaultCfg)
	b := hasher.Compute([]string{"groq/llama-3.3-70b-versatile", "openai/gpt-4o"}, sampleDS, defaultCfg)
	if a != b {
		t.Errorf("hash differs by model order: %q vs %q", a, b)
	}
}

func TestCompute_DiffersOnDifferentModels(t *testing.T) {
	h1 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, defaultCfg)
	h2 := hasher.Compute([]string{"anthropic/claude-haiku-4-5"}, sampleDS, defaultCfg)
	if h1 == h2 {
		t.Error("different models produced same hash")
	}
}

func TestCompute_DiffersOnDifferentTask(t *testing.T) {
	ds2 := sampleDS
	ds2.Task = "Different task"
	h1 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, defaultCfg)
	h2 := hasher.Compute([]string{"openai/gpt-4o"}, ds2, defaultCfg)
	if h1 == h2 {
		t.Error("different tasks produced same hash")
	}
}

func TestCompute_DiffersOnDifferentTemperature(t *testing.T) {
	cfg2 := config.ModelConfig{MaxTokens: 1024, Temperature: 0.0}
	h1 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, defaultCfg)
	h2 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, cfg2)
	if h1 == h2 {
		t.Error("different temperature produced same hash")
	}
}

func TestCompute_DiffersOnDifferentMaxTokens(t *testing.T) {
	cfg2 := config.ModelConfig{MaxTokens: 512, Temperature: 0.7}
	h1 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, defaultCfg)
	h2 := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, cfg2)
	if h1 == h2 {
		t.Error("different max_tokens produced same hash")
	}
}

func TestCompute_Length(t *testing.T) {
	h := hasher.Compute([]string{"openai/gpt-4o"}, sampleDS, defaultCfg)
	if len(h) != 32 {
		t.Errorf("hash length = %d, want 32", len(h))
	}
}
