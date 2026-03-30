package providers

import (
	"context"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
)

// Provider is the extension point for LLM backends. A single-method interface keeps the
// runner, harden loop, and tests working against a minimal surface — a new backend is a
// ~50-line file plus one registration line in registry.go.
type Provider interface {
	// Complete is the single unit of work: one prompt in, one response out. Errors are
	// returned rather than logged so callers decide whether to abort or record and continue.
	Complete(ctx context.Context, modelName string, prompt dataset.Prompt, cfg config.ModelConfig) (dataset.Response, error)
}
