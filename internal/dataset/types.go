package dataset

import "fmt"

// Prompt is a single test case sent to each model.
type Prompt struct {
	ID     string `json:"id" yaml:"id"`
	System string `json:"system,omitempty" yaml:"system,omitempty"`
	User   string `json:"user" yaml:"user"`
}

// Dataset is the generated list of test prompts for a task.
type Dataset struct {
	Task    string   `json:"task" yaml:"task"`
	Params  []string `json:"params" yaml:"params"`
	Prompts []Prompt `json:"prompts" yaml:"prompts"`
}

// Tokens holds token usage counts.
type Tokens struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Response is one model's output for one prompt.
type Response struct {
	Model     string `json:"model"`
	PromptID  string `json:"prompt_id"`
	Text      string `json:"text"`
	LatencyMs int64  `json:"latency_ms"`
	Tokens    Tokens `json:"tokens"`
	Error     string `json:"error,omitempty"`
}

// RawOutput is the file produced by collect_responses.
type RawOutput struct {
	Hash      string     `json:"hash"`
	Dataset   Dataset    `json:"dataset"`
	Models    []string   `json:"models"`
	Responses []Response `json:"responses"`
}

// SafeText returns the model output wrapped in unambiguous delimiters for safe
// embedding in judge or scorer prompts.
//
// IMPORTANT: Never embed Response.Text directly when constructing evaluation
// prompts — an adversarial dataset author can place instructions in the user
// field that the model echoes, which then manipulate the judge's scoring.
// Always use SafeText() when building judge prompts to prevent this injection.
//
// Example scorer usage:
//
//	judgePrompt := fmt.Sprintf("Score this response:\n%s", r.SafeText())
func (r Response) SafeText() string {
	return fmt.Sprintf(
		"<<<BEGIN_MODEL_OUTPUT prompt_id=%q model=%q>>>\n%s\n<<<END_MODEL_OUTPUT>>>",
		r.PromptID, r.Model, r.Text,
	)
}
