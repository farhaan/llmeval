package runner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
	"github.com/farhaan/llmeval/internal/providers"
	"github.com/farhaan/llmeval/internal/runner"
)

// okHandler returns a minimal OpenAI-compat response.
func okHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": text}},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 3},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func buildRegistry(t *testing.T, providerName, baseURL string) *providers.Registry {
	t.Helper()
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			providerName: {
				Type:    config.ProviderOpenAICompat,
				BaseURL: baseURL,
			},
		},
	}
	reg, err := providers.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}

var testCfg = config.ModelConfig{MaxTokens: 256, Temperature: 0.5}

func TestRun_SingleModelSinglePrompt(t *testing.T) {
	srv := httptest.NewServer(okHandler("hello"))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)
	ds := dataset.Dataset{
		Prompts: []dataset.Prompt{{ID: "p1", User: "hi"}},
	}

	results := runner.Run(context.Background(), reg, []string{"p/model"}, ds, testCfg, 2)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Text != "hello" {
		t.Errorf("Text = %q, want hello", results[0].Text)
	}
	if results[0].Model != "p/model" {
		t.Errorf("Model = %q, want p/model", results[0].Model)
	}
	if results[0].Error != "" {
		t.Errorf("unexpected error: %s", results[0].Error)
	}
}

func TestRun_MultipleModelsMultiplePrompts(t *testing.T) {
	srv := httptest.NewServer(okHandler("ok"))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)
	ds := dataset.Dataset{
		Prompts: []dataset.Prompt{
			{ID: "p1", User: "q1"},
			{ID: "p2", User: "q2"},
		},
	}

	results := runner.Run(context.Background(), reg, []string{"p/m1", "p/m2"}, ds, testCfg, 4)
	// 2 models × 2 prompts = 4 results
	if len(results) != 4 {
		t.Fatalf("len(results) = %d, want 4", len(results))
	}
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("unexpected error for %s/%s: %s", r.Model, r.PromptID, r.Error)
		}
	}
}

func TestRun_UnresolvableModel_RecordsError(t *testing.T) {
	srv := httptest.NewServer(okHandler("ok"))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)
	ds := dataset.Dataset{
		Prompts: []dataset.Prompt{{ID: "p1", User: "q"}},
	}

	results := runner.Run(context.Background(), reg, []string{"unknown/model"}, ds, testCfg, 2)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error for unknown provider, got empty string")
	}
	if results[0].Model != "unknown/model" {
		t.Errorf("Model = %q, want unknown/model", results[0].Model)
	}
}

func TestRun_CancelledContext_RecordsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)
	ds := dataset.Dataset{
		Prompts: []dataset.Prompt{{ID: "p1", User: "q"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := runner.Run(ctx, reg, []string{"p/model"}, ds, testCfg, 2)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error for cancelled context")
	}
}

func TestRun_ConcurrencyLimitRespected(t *testing.T) {
	var inflight atomic.Int32
	var maxSeen atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inflight.Add(1)
		defer inflight.Add(-1)
		// Track max concurrent calls.
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		okHandler("ok")(w, r)
	}))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)

	const numPrompts = 10
	prompts := make([]dataset.Prompt, numPrompts)
	for i := range prompts {
		prompts[i] = dataset.Prompt{ID: "p", User: "q"}
	}
	ds := dataset.Dataset{Prompts: prompts}

	const concurrency = 3
	runner.Run(context.Background(), reg, []string{"p/model"}, ds, testCfg, concurrency)

	if got := maxSeen.Load(); got > int32(concurrency) {
		t.Errorf("max concurrent calls = %d, exceeds concurrency limit %d", got, concurrency)
	}
}

func TestRun_PreservesOrder(t *testing.T) {
	srv := httptest.NewServer(okHandler("ok"))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)

	prompts := []dataset.Prompt{
		{ID: "first", User: "q"},
		{ID: "second", User: "q"},
		{ID: "third", User: "q"},
	}
	ds := dataset.Dataset{Prompts: prompts}

	results := runner.Run(context.Background(), reg, []string{"p/model"}, ds, testCfg, 4)
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	// Results must appear in job order (model×prompt iteration order).
	ids := []string{results[0].PromptID, results[1].PromptID, results[2].PromptID}
	want := []string{"first", "second", "third"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("results[%d].PromptID = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestRun_EmptyDataset(t *testing.T) {
	srv := httptest.NewServer(okHandler("ok"))
	defer srv.Close()

	reg := buildRegistry(t, "p", srv.URL)
	ds := dataset.Dataset{Prompts: nil}

	results := runner.Run(context.Background(), reg, []string{"p/model"}, ds, testCfg, 2)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty dataset, got %d", len(results))
	}
}
