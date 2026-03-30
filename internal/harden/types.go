package harden

import (
	"fmt"

	"github.com/farhaan/llmeval/internal/config"
)

// Probe is one adversarial test case for the target model.
type Probe struct {
	ID        string `json:"id"`
	System    string `json:"system,omitempty"` // filled system prompt (indirect injection probes set custom system)
	User      string `json:"user"`
	Category  string `json:"category"` // role_override | indirect_injection | hallucination | etc.
	Rationale string `json:"rationale"`
}

// Finding is one jailbreak result from the judge.
type Finding struct {
	ProbeID     string `json:"probe_id"`
	Probe       string `json:"probe"`
	Response    string `json:"response"`
	Jailbroken  bool   `json:"jailbroken"`
	Type        string `json:"type"`     // role_override | system_prompt_leak | indirect_injection | hallucination | format_break
	Severity    string `json:"severity"` // CRITICAL | MAJOR | MINOR
	Explanation string `json:"explanation"`
}

// PatchOp is one surgical edit to apply to the prompt file.
type PatchOp struct {
	Find    string `json:"find"`    // exact string to find in prompt file
	Replace string `json:"replace"` // replacement string
}

// GoldenResult is one baseline query result after patching.
type GoldenResult struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Response string `json:"response"`
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason,omitempty"`
}

// IterFindings bundles one iteration's findings for passing to analyze_history.
type IterFindings struct {
	Iteration int       `json:"iteration"`
	Findings  []Finding `json:"findings"`
}

// IterPatch bundles one iteration's patch for passing to analyze_history.
type IterPatch struct {
	Iteration   int       `json:"iteration"`
	Ops         []PatchOp `json:"ops"`
	Explanation string    `json:"explanation"`
}

// IterState is the checkpoint for one iteration (saved to disk).
type IterState struct {
	StepDone             string `json:"step_done"` // latest step completed
	ProbesCount          int    `json:"probes_count"`
	FindingsCount        *int   `json:"findings_count,omitempty"`         // nil until judged
	ConfirmFindingsCount *int   `json:"confirm_findings_count,omitempty"` // nil until confirmation judged
	GoldenPassed         *int   `json:"golden_passed,omitempty"`
}

// RunState is the top-level checkpoint for one prompt's harden run.
type RunState struct {
	Prompt            string `json:"prompt"`
	Target            string `json:"target"`
	PromptHash        string `json:"prompt_hash"`
	StartedAt         string `json:"started_at"`
	CurrentIter       int    `json:"current_iter"`
	LastCompletedIter int    `json:"last_completed_iter,omitempty"`
	Status            string `json:"status,omitempty"` // "clean|stuck|reverted"
}

// PromptEntry is one entry from the prompt manifest YAML.
type PromptEntry struct {
	Path           string              `yaml:"path"`
	Name           string              `yaml:"name"`
	Description    string              `yaml:"description"`
	RiskProfile    string              `yaml:"risk_profile"` // direct_injection | indirect_injection | output_fidelity | hallucination
	ExpectedOutput string              `yaml:"expected_output"`
	Fixtures       map[string]string   `yaml:"fixtures"`
	GoldenQueries  []string            `yaml:"golden_queries"` // optional: baseline user queries for patch validation; auto-generated if empty
	ContextPaths   []string            `yaml:"context_paths"`  // optional: file globs for codebase context injected into probe generation; if empty, Claude auto-researches
	Targets        []string            `yaml:"models"`         // optional: target models for this prompt; overrides harden.target (e.g. ["groq/llama-3.3-70b", "anthropic/claude-sonnet-4-6"])
	SkipGolden     *bool               `yaml:"skip_golden"`    // optional: override global skip_golden for this prompt (nil = inherit)
	Cache          *bool               `yaml:"cache"`          // optional: override global cache for this prompt (nil = inherit)
	ModelConfig    *config.ModelConfig `yaml:"model_config"`   // optional: override max_tokens/temperature for this prompt's target model (e.g. guard models with tiny context windows)
}

// PromptManifest is the top-level manifest YAML.
type PromptManifest struct {
	Prompts []PromptEntry `yaml:"prompts"`
}

// Report is the final summary for one prompt's harden run.
type Report struct {
	Prompt        string
	Name          string
	TotalIters    int
	TotalPatches  int
	FinalFindings int
	Status        string // "clean|stuck|reverted"
}

// AIRequest is written to claude_request.json when the binary needs the skill
// to perform an AI step. The skill reads this, calls Claude, writes the output
// file at OutputFile, then resumes the binary with --resume.
type AIRequest struct {
	Type       string `json:"type"`        // research_context | analyze_history | linguistic_expert_analysis | generate_probes | judge | generate_patch | generate_golden_queries
	IterDir    string `json:"iter_dir"`    // path to this iteration's checkpoint dir
	OutputFile string `json:"output_file"` // absolute path the skill must write the result to

	// thinking/reasoning — if >0, the skill should apply extended reasoning before generating output
	ThinkingBudget int `json:"thinking_budget,omitempty"`

	// analyze_history
	AllFindingsHistory []IterFindings `json:"all_findings_history,omitempty"`
	AllPatches         []IterPatch    `json:"all_patches,omitempty"`

	// linguistic_expert_analysis
	Language string `json:"language,omitempty"` // BCP-47 code, e.g. "en", "id", "ja"

	// generate_probes
	Count           int               `json:"count,omitempty"`
	RiskProfile     string            `json:"risk_profile,omitempty"`
	PromptName      string            `json:"prompt_name,omitempty"`
	Description     string            `json:"description,omitempty"`
	FilledSystem    string            `json:"filled_system,omitempty"`
	Iteration       int               `json:"iteration,omitempty"`
	Fixtures        map[string]string `json:"fixtures,omitempty"`
	AllFindingsFlat []Finding         `json:"all_findings_flat,omitempty"` // deduplicated findings across all iterations (for generate_probes context)
	CodebaseContext string            `json:"codebase_context,omitempty"`  // relevant app code snippets
	Strategy        string            `json:"strategy,omitempty"`          // JSON from analyze_history step
	CulturalContext string            `json:"cultural_context,omitempty"`  // JSON from linguistic_expert_analysis step

	// judge
	Probes    []Probe           `json:"probes,omitempty"`
	Responses map[string]string `json:"responses,omitempty"`

	// generate_patch
	PromptContent string    `json:"prompt_content,omitempty"`
	Findings      []Finding `json:"findings,omitempty"`

	// generate_golden_queries
	ExpectedOutput string `json:"expected_output,omitempty"`
}

// AINeededError is returned by Run when the binary needs the skill to perform
// an AI step. The CLI should print RequestFile and exit with code 10.
type AINeededError struct {
	RequestFile string
}

func (e *AINeededError) Error() string {
	return fmt.Sprintf("AI assistance needed: %s", e.RequestFile)
}
