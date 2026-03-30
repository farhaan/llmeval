package runner

import (
	"context"
	"strings"
	"sync"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
	"github.com/farhaan/llmeval/internal/providers"
)

// Run fans out all (model × prompt) pairs concurrently.
//
// Concurrency is enforced per-provider: each provider gets its own semaphore
// of size `concurrency`. This prevents a slow provider (e.g. a local Ollama
// instance) from monopolising all slots and blocking calls to faster providers.
//
// Failed calls are recorded in Response.Error rather than halting the run.
func Run(ctx context.Context, reg *providers.Registry, models []string, ds dataset.Dataset, cfg config.ModelConfig, concurrency int) []dataset.Response {
	type job struct {
		model  string
		prompt dataset.Prompt
	}

	jobs := make([]job, 0, len(models)*len(ds.Prompts))
	for _, m := range models {
		for _, p := range ds.Prompts {
			jobs = append(jobs, job{model: m, prompt: p})
		}
	}

	// Build one semaphore per provider so that each provider's concurrency is
	// capped independently. A slow provider cannot starve a fast one.
	sems := make(map[string]chan struct{})
	for _, m := range models {
		p := providerPrefix(m)
		if _, ok := sems[p]; !ok {
			sems[p] = make(chan struct{}, concurrency)
		}
	}

	results := make([]dataset.Response, len(jobs))
	var wg sync.WaitGroup

	for i, j := range jobs {
		wg.Add(1)
		go func(idx int, jb job) {
			defer wg.Done()

			sem := sems[providerPrefix(jb.model)]
			sem <- struct{}{}
			defer func() { <-sem }()

			provider, modelName, err := reg.Resolve(jb.model)
			if err != nil {
				results[idx] = dataset.Response{
					Model:    jb.model,
					PromptID: jb.prompt.ID,
					Error:    err.Error(),
				}
				return
			}

			resp, err := provider.Complete(ctx, modelName, jb.prompt, cfg)
			resp.Model = jb.model // always expose the user-facing ID, not the internal model name
			if err != nil {
				resp.Error = err.Error()
			}
			results[idx] = resp
		}(i, j)
	}

	wg.Wait()
	return results
}

// providerPrefix maps a model ID to its provider key for semaphore lookup so per-provider
// concurrency caps hold regardless of whether the caller uses prefix syntax ("groq/model")
// or a config alias that doesn't contain a slash.
func providerPrefix(modelID string) string {
	if prefix, _, found := strings.Cut(modelID, "/"); found {
		return prefix
	}
	return modelID
}
