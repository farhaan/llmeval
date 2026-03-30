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

// OpenAICompat is one struct for every provider that speaks the OpenAI chat completions protocol,
// rather than a separate type per provider. New backends (Ollama, LM Studio, any proxy) are added
// via config alone — no code changes required.
type OpenAICompat struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

const defaultTimeoutSecs = 120

func NewOpenAICompat(baseURL, apiKey string, timeoutSecs int) *OpenAICompat {
	if timeoutSecs <= 0 {
		timeoutSecs = defaultTimeoutSecs
	}
	return &OpenAICompat{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second},
	}
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAICompat) Complete(ctx context.Context, modelName string, prompt dataset.Prompt, cfg config.ModelConfig) (dataset.Response, error) {
	resp := dataset.Response{PromptID: prompt.ID}

	msgs := make([]oaiMessage, 0, 2)
	if prompt.System != "" {
		msgs = append(msgs, oaiMessage{Role: "system", Content: prompt.System})
	}
	msgs = append(msgs, oaiMessage{Role: "user", Content: prompt.User})

	body, _ := json.Marshal(oaiRequest{
		Model:       modelName,
		Messages:    msgs,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	start := time.Now()
	httpResp, err := o.client.Do(req)
	if err != nil {
		return resp, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	resp.LatencyMs = time.Since(start).Milliseconds()

	if ct := httpResp.Header.Get("Content-Type"); ct != "" && strings.Contains(ct, "text/event-stream") {
		return resp, fmt.Errorf("provider returned a streaming response (Content-Type: %s) — set stream: false or use a non-streaming endpoint", ct)
	}

	var ar oaiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&ar); err != nil {
		return resp, fmt.Errorf("decode: %w", err)
	}
	if ar.Error != nil {
		return resp, fmt.Errorf("API error: %s", ar.Error.Message)
	}
	if len(ar.Choices) == 0 {
		return resp, fmt.Errorf("empty choices (status %d)", httpResp.StatusCode)
	}

	resp.Text = ar.Choices[0].Message.Content
	resp.Tokens.Input = ar.Usage.PromptTokens
	resp.Tokens.Output = ar.Usage.CompletionTokens
	return resp, nil
}
