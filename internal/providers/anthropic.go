package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
)

// Anthropic is a separate provider rather than an OpenAICompat instance because the two APIs
// differ in shape: system prompts are a top-level field (not a message), authentication uses
// x-api-key instead of Bearer, and a shared adapter for both would add branching that makes
// each path harder to test in isolation.
type Anthropic struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewAnthropic(baseURL, apiKey string, timeoutSecs int) *Anthropic {
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	if timeoutSecs <= 0 {
		timeoutSecs = defaultTimeoutSecs
	}
	return &Anthropic{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second},
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Complete(ctx context.Context, modelName string, prompt dataset.Prompt, cfg config.ModelConfig) (dataset.Response, error) {
	resp := dataset.Response{PromptID: prompt.ID}

	body, _ := json.Marshal(anthropicRequest{
		Model:       modelName,
		System:      prompt.System,
		Messages:    []anthropicMessage{{Role: "user", Content: prompt.User}},
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	start := time.Now()
	httpResp, err := a.client.Do(req)
	if err != nil {
		return resp, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	resp.LatencyMs = time.Since(start).Milliseconds()

	var ar anthropicResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&ar); err != nil {
		return resp, fmt.Errorf("decode: %w", err)
	}
	if ar.Error != nil {
		return resp, fmt.Errorf("API error: %s", ar.Error.Message)
	}
	if len(ar.Content) == 0 {
		return resp, fmt.Errorf("empty content (status %d)", httpResp.StatusCode)
	}

	resp.Text = ar.Content[0].Text
	resp.Tokens.Input = ar.Usage.InputTokens
	resp.Tokens.Output = ar.Usage.OutputTokens
	return resp, nil
}
