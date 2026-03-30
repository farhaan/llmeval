package providers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
	"github.com/farhaan/llmeval/internal/providers"
)

var testCfg = config.ModelConfig{MaxTokens: 512, Temperature: 0.5}
var testPrompt = dataset.Prompt{ID: "p1", System: "Be concise.", User: "Hello"}

// ── OpenAICompat ──────────────────────────────────────────────────────────────

func TestOpenAICompat_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "chat/completions") {
			t.Errorf("path = %s, want .../chat/completions", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "my-model" {
			t.Errorf("model = %v", req["model"])
		}
		if req["temperature"].(float64) != 0.5 {
			t.Errorf("temperature = %v", req["temperature"])
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "Hi there!"}},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "test-key", 0)
	resp, err := p.Complete(context.Background(), "my-model", testPrompt, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hi there!" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.Tokens.Input != 10 || resp.Tokens.Output != 5 {
		t.Errorf("tokens = %+v", resp.Tokens)
	}
	if resp.PromptID != "p1" {
		t.Errorf("prompt_id = %q", resp.PromptID)
	}
}

func TestOpenAICompat_NoAPIKey_OmitsAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
			"usage":   map[string]any{},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "", 0)
	if _, err := p.Complete(context.Background(), "model", testPrompt, testCfg); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAICompat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "invalid api key"},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "bad-key", 0)
	_, err := p.Complete(context.Background(), "model", testPrompt, testCfg)
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("expected API error, got %v", err)
	}
}

func TestOpenAICompat_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []any{}}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "key", 0)
	_, err := p.Complete(context.Background(), "model", testPrompt, testCfg)
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestOpenAICompat_StreamingResponse_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "key", 0)
	_, err := p.Complete(context.Background(), "model", testPrompt, testCfg)
	if err == nil || !strings.Contains(err.Error(), "streaming") {
		t.Errorf("expected streaming error, got %v", err)
	}
}

func TestOpenAICompat_SystemPromptIncluded(t *testing.T) {
	var gotMessages []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		msgs, _ := req["messages"].([]any)
		for _, m := range msgs {
			gotMessages = append(gotMessages, m.(map[string]any))
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
			"usage":   map[string]any{},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewOpenAICompat(srv.URL, "key", 0)
	_, _ = p.Complete(context.Background(), "model", testPrompt, testCfg)

	if len(gotMessages) != 2 {
		t.Fatalf("messages = %d, want 2 (system + user)", len(gotMessages))
	}
	if gotMessages[0]["role"] != "system" {
		t.Errorf("first role = %v, want system", gotMessages[0]["role"])
	}
	if gotMessages[1]["role"] != "user" {
		t.Errorf("second role = %v, want user", gotMessages[1]["role"])
	}
}

func TestOpenAICompat_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// never respond
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	p := providers.NewOpenAICompat(srv.URL, "key", 0)
	_, err := p.Complete(ctx, "model", testPrompt, testCfg)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

func TestAnthropic_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ant-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header missing")
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "claude-test" {
			t.Errorf("model = %v", req["model"])
		}
		if req["system"] != "Be concise." {
			t.Errorf("system = %v", req["system"])
		}
		if req["temperature"].(float64) != 0.5 {
			t.Errorf("temperature = %v", req["temperature"])
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Hello back!"}},
			"usage":   map[string]any{"input_tokens": 8, "output_tokens": 3},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewAnthropic(srv.URL, "ant-key", 0)
	resp, err := p.Complete(context.Background(), "claude-test", testPrompt, testCfg)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hello back!" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.Tokens.Input != 8 || resp.Tokens.Output != 3 {
		t.Errorf("tokens = %+v", resp.Tokens)
	}
}

func TestAnthropic_DefaultBaseURL(t *testing.T) {
	// NewAnthropic with empty baseURL should not panic; real URL is used in prod.
	p := providers.NewAnthropic("", "key", 0)
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestAnthropic_CustomBaseURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if err := json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "proxied"}},
			"usage":   map[string]any{},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewAnthropic(srv.URL, "key", 0)
	_, _ = p.Complete(context.Background(), "model", testPrompt, testCfg)
	if !called {
		t.Error("custom base URL not used")
	}
}

func TestAnthropic_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "model not found"},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewAnthropic(srv.URL, "key", 0)
	_, err := p.Complete(context.Background(), "bad-model", testPrompt, testCfg)
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Errorf("expected API error, got %v", err)
	}
}

func TestAnthropic_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"content": []any{}}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	p := providers.NewAnthropic(srv.URL, "key", 0)
	_, err := p.Complete(context.Background(), "model", testPrompt, testCfg)
	if err == nil {
		t.Error("expected error for empty content")
	}
}
