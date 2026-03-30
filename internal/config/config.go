package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderOpenAICompat ProviderType = "openai_compat"
	ProviderAnthropic    ProviderType = "anthropic"
)

// ProviderConfig describes one named provider endpoint. Keeping type and base_url here rather
// than hardcoding them in provider constructors means users can add or reconfigure providers
// (including local ones like Ollama) via YAML alone, with no code changes.
type ProviderConfig struct {
	Type        ProviderType `yaml:"type"`
	BaseURL     string       `yaml:"base_url"`
	APIKeyEnv   string       `yaml:"api_key_env"`  // override env var name; empty = derive from provider name
	TimeoutSecs int          `yaml:"timeout_secs"` // HTTP client timeout in seconds; 0 = use default (120s)
	Enabled     *bool        `yaml:"enabled"`      // nil or true = enabled; explicit false = disabled (removes from registry)
}

// EnvVar derives the key name from a convention (UPPER_API_KEY) so most providers work
// without explicit api_key_env config. An override is only needed when the key lives under
// a non-standard name (e.g. an existing CI secret that predates llmeval).
func (pc ProviderConfig) EnvVar(providerName string) string {
	if pc.APIKeyEnv != "" {
		return pc.APIKeyEnv
	}
	return strings.ToUpper(providerName) + "_API_KEY"
}

// ModelMapping separates user-facing model aliases from provider-specific names. If a provider
// renames a model, only the mapping needs updating — no call sites change. If a model ID is
// not listed here, prefix routing is used ("groq/llama-3.3-70b" → provider "groq").
type ModelMapping struct {
	ID        string `yaml:"id"`         // e.g. "groq/llama-3.3-70b"
	Provider  string `yaml:"provider"`   // must match a key in Providers
	ModelName string `yaml:"model_name"` // name sent to the API
}

// ModelConfig holds per-request generation parameters.
type ModelConfig struct {
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
}

// TaskConfig defines a named, reusable evaluation scenario.
// Run with: /llmeval --task-id <name>  or  llmeval collect --task-id <name>
//
// CLI flags always override task-level defaults, so tasks act as presets
// rather than hard constraints.
type TaskConfig struct {
	// Description is the task prompt passed to Claude for dataset generation.
	// Equivalent to /llmeval --task "..."
	Description string `yaml:"description"`

	// System is the default system prompt injected into every generated prompt.
	// Individual prompts in a hand-crafted dataset can override this.
	System string `yaml:"system"`

	// Params are the scoring dimensions for pointwise mode.
	// Omit to let Claude auto-select based on the description.
	Params []string `yaml:"params"`

	// Models to compare. Overridden by --models flag.
	Models []string `yaml:"models"`

	// DatasetSize overrides the global default for this task.
	DatasetSize int `yaml:"dataset_size"`

	// Mode is "pointwise" (default) or "pairwise".
	Mode string `yaml:"mode"`

	// Concurrency overrides the global default for this task.
	Concurrency int `yaml:"concurrency"`
}

// HardenConfig groups harden settings so they are loaded from YAML and overridden by CLI flags
// in one place. The loop itself reads from RunConfig, never from HardenConfig directly, keeping
// the YAML schema decoupled from runtime state.
type HardenConfig struct {
	Manifest         string    `yaml:"manifest"`          // path to prompt manifest YAML
	Target           string    `yaml:"target"`            // model to attack, e.g. "groq/openai/gpt-oss-120b"
	JudgeModel       string    `yaml:"judge_model"`       // model for probe gen+judge, e.g. "anthropic/claude-sonnet-4-6"
	Probes           int       `yaml:"probes"`            // probes per iteration, default 15
	Concurrency      int       `yaml:"concurrency"`       // parallel API calls, default 5
	MaxIterations    int       `yaml:"max_iterations"`    // 0 = unlimited
	Until            StopConds `yaml:"until"`             // stop conditions
	RunDir           string    `yaml:"run_dir"`           // default ".llmeval/harden"
	AuditLog         string    `yaml:"audit_log"`         // default ".llmeval/harden_audit.jsonl"
	ConfirmationRuns int       `yaml:"confirmation_runs"` // re-runs when findings==0 to confirm clean; default 1
	ProbeRuns        int       `yaml:"probe_runs"`        // runs per probe for noise reduction; default 1
	SkipGolden       bool      `yaml:"skip_golden"`       // skip golden validation after patching
	Cache            bool      `yaml:"cache"`             // skip run if prompt hash unchanged from last completed run
}

// StopConds makes termination criteria explicit and serialisable so a checkpoint carries the
// original stop intent and --resume can honour it without re-parsing CLI flags.
type StopConds struct {
	FindingsEq *int `yaml:"findings_eq"` // stop when findings count == this value (usually 0)
}

// Config is the merged result of built-in defaults + user YAML. Defaults live in code rather
// than a file so an absent or minimal config still produces a working state.
type Config struct {
	Providers   map[string]ProviderConfig `yaml:"providers"`
	Models      []ModelMapping            `yaml:"models"`
	Tasks       map[string]TaskConfig     `yaml:"tasks"`
	DatasetSize int                       `yaml:"dataset_size"`
	Concurrency int                       `yaml:"concurrency"`
	ModelConfig ModelConfig               `yaml:"model_config"`
	Harden      HardenConfig              `yaml:"harden"`
}

// defaultFindingsEq is the default stop condition: stop when findings == 0.
var defaultFindingsEq = 0

// Default returns a Config with all built-in providers pre-wired.
// Users only need a yaml file to override or extend this.
func Default() *Config {
	return &Config{
		DatasetSize: 5,
		Concurrency: 5,
		Tasks:       make(map[string]TaskConfig),
		ModelConfig: ModelConfig{
			MaxTokens:   1024,
			Temperature: 0.7,
		},
		Providers: map[string]ProviderConfig{
			"groq":       {Type: ProviderOpenAICompat, BaseURL: "https://api.groq.com/openai/v1"},
			"openai":     {Type: ProviderOpenAICompat, BaseURL: "https://api.openai.com/v1"},
			"openrouter": {Type: ProviderOpenAICompat, BaseURL: "https://openrouter.ai/api/v1"},
			"deepinfra":  {Type: ProviderOpenAICompat, BaseURL: "https://api.deepinfra.com/v1/openai"},
			"anthropic":  {Type: ProviderAnthropic},
		},
		Harden: HardenConfig{
			Probes:           15,
			Concurrency:      5,
			MaxIterations:    5,
			RunDir:           ".llmeval/harden",
			AuditLog:         ".llmeval/harden_audit.jsonl",
			Until:            StopConds{FindingsEq: &defaultFindingsEq},
			ConfirmationRuns: 1,
			ProbeRuns:        1,
		},
	}
}

// Load merges user YAML over built-in defaults field-by-field so a config file only needs
// to express what differs from conventions. An empty path is valid and returns defaults
// unchanged, making the CLI work without any config file present.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var override Config
	if err := yaml.Unmarshal(data, &override); err != nil {
		return nil, err
	}
	mergeProviders(cfg, override.Providers)
	if len(override.Models) > 0 {
		cfg.Models = override.Models
	}
	for id, tc := range override.Tasks {
		cfg.Tasks[id] = tc
	}
	if override.DatasetSize > 0 {
		cfg.DatasetSize = override.DatasetSize
	}
	if override.Concurrency > 0 {
		cfg.Concurrency = override.Concurrency
	}
	mergeModelConfig(&cfg.ModelConfig, override.ModelConfig)
	mergeHardenConfig(&cfg.Harden, override.Harden)
	return cfg, nil
}

func mergeProviders(cfg *Config, overrides map[string]ProviderConfig) {
	for name, pc := range overrides {
		if pc.Enabled != nil && !*pc.Enabled {
			delete(cfg.Providers, name)
			continue
		}
		cfg.Providers[name] = pc
	}
}

func mergeModelConfig(dst *ModelConfig, src ModelConfig) {
	if src.MaxTokens > 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.Temperature > 0 {
		dst.Temperature = src.Temperature
	}
}

func mergeHardenConfig(dst *HardenConfig, h HardenConfig) {
	if h.Manifest != "" {
		dst.Manifest = h.Manifest
	}
	if h.Target != "" {
		dst.Target = h.Target
	}
	if h.JudgeModel != "" {
		dst.JudgeModel = h.JudgeModel
	}
	if h.Probes > 0 {
		dst.Probes = h.Probes
	}
	if h.Concurrency > 0 {
		dst.Concurrency = h.Concurrency
	}
	if h.MaxIterations != 0 {
		dst.MaxIterations = h.MaxIterations
	}
	if h.Until.FindingsEq != nil {
		dst.Until.FindingsEq = h.Until.FindingsEq
	}
	mergeHardenStrings(dst, h)
	mergeHardenCounts(dst, h)
	mergeHardenBools(dst, h)
}

func mergeHardenStrings(dst *HardenConfig, h HardenConfig) {
	if h.RunDir != "" {
		dst.RunDir = h.RunDir
	}
	if h.AuditLog != "" {
		dst.AuditLog = h.AuditLog
	}
}

func mergeHardenCounts(dst *HardenConfig, h HardenConfig) {
	if h.ConfirmationRuns > 0 {
		dst.ConfirmationRuns = h.ConfirmationRuns
	}
	if h.ProbeRuns > 0 {
		dst.ProbeRuns = h.ProbeRuns
	}
}

func mergeHardenBools(dst *HardenConfig, h HardenConfig) {
	if h.SkipGolden {
		dst.SkipGolden = true
	}
	if h.Cache {
		dst.Cache = true
	}
}
