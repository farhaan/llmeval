package providers

import (
	"fmt"
	"os"
	"strings"

	"github.com/farhaan/llmeval/internal/config"
)

// Registry routes model IDs to Provider instances.
// Resolution order:
//  1. Explicit ModelMapping in config (id → provider + model_name override)
//  2. Prefix fallback: "groq/llama-3.3-70b" → provider "groq", modelName "llama-3.3-70b"
type Registry struct {
	providers   map[string]Provider            // key: provider name e.g. "groq"
	models      map[string]config.ModelMapping // key: model ID e.g. "groq/llama-3.3-70b"
	missingKeys []string                       // provider names whose API key env var was empty
}

// NewRegistry builds the provider map from config so the rest of the codebase never reads
// os.Getenv directly. Collecting missing-key names here rather than failing lets the caller
// emit a single pre-flight warning instead of discovering each gap as a 401 mid-run.
func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		providers: make(map[string]Provider),
		models:    make(map[string]config.ModelMapping),
	}

	for name, pc := range cfg.Providers {
		apiKey := os.Getenv(pc.EnvVar(name))
		if apiKey == "" {
			r.missingKeys = append(r.missingKeys, name)
		}
		timeout := pc.TimeoutSecs
		switch pc.Type {
		case config.ProviderOpenAICompat:
			r.providers[name] = NewOpenAICompat(pc.BaseURL, apiKey, timeout)
		case config.ProviderAnthropic:
			r.providers[name] = NewAnthropic(pc.BaseURL, apiKey, timeout)
		default:
			return nil, fmt.Errorf("unknown provider type %q for %q", pc.Type, name)
		}
	}

	for _, m := range cfg.Models {
		r.models[m.ID] = m
	}

	return r, nil
}

// Resolve decouples user-facing model IDs ("groq/llama-3.3-70b") from the name the
// provider API expects. Users can rename or alias models in config without changing call sites.
func (r *Registry) Resolve(modelID string) (Provider, string, error) {
	// Explicit mapping wins.
	if m, ok := r.models[modelID]; ok {
		p, ok := r.providers[m.Provider]
		if !ok {
			return nil, "", fmt.Errorf("provider %q not configured (referenced by model %q)", m.Provider, modelID)
		}
		return p, m.ModelName, nil
	}

	// Prefix fallback: "<provider>/<model-name>"
	prefix, modelName, found := strings.Cut(modelID, "/")
	if found {
		if p, ok := r.providers[prefix]; ok {
			return p, modelName, nil
		}
	}

	return nil, "", fmt.Errorf(
		"no provider found for %q — define it in llmeval.yaml or use <provider>/<model> format (known providers: %s)",
		modelID, r.knownProviders(),
	)
}

// ValidateModels surfaces typos before any API calls start — a mistyped model ID fails fast
// rather than letting 99 valid requests succeed before one fails with a confusing 404.
func (r *Registry) ValidateModels(models []string) []error {
	var errs []error
	for _, id := range models {
		if _, _, err := r.Resolve(id); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// MissingKeys lets callers emit a single pre-flight warning rather than discovering absent
// keys one by one as 401 errors arrive during the run.
func (r *Registry) MissingKeys() []string {
	return r.missingKeys
}

func (r *Registry) knownProviders() string {
	names := make([]string, 0, len(r.providers))
	for k := range r.providers {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
