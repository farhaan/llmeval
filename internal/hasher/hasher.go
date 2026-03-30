package hasher

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
)

// Compute returns a stable 32-char hex hash of (models, dataset, cfg) for cache keying.
// 32 hex chars = 128 bits — negligible collision probability even at scale.
// Sorting models ensures order doesn't affect the hash.
// ModelConfig is included so temperature/max_tokens changes invalidate the cache.
func Compute(models []string, ds dataset.Dataset, cfg config.ModelConfig) string {
	sorted := make([]string, len(models))
	copy(sorted, models)
	sort.Strings(sorted)

	payload := struct {
		Models      []string         `json:"models"`
		Task        string           `json:"task"`
		Prompts     []dataset.Prompt `json:"prompts"`
		MaxTokens   int              `json:"max_tokens"`
		Temperature float64          `json:"temperature"`
	}{
		Models:      sorted,
		Task:        ds.Task,
		Prompts:     ds.Prompts,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
	}

	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)[:32]
}
