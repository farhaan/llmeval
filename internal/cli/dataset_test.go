package cli

import (
	"testing"
)

// loadDataset is an unexported function tested via the package-internal test.

func TestLoadDataset_JSON(t *testing.T) {
	raw := []byte(`{
		"task": "Explain recursion",
		"params": ["clarity", "accuracy"],
		"prompts": [
			{"id": "p1", "user": "What is recursion?"},
			{"id": "p2", "system": "Be concise.", "user": "Give an example."}
		]
	}`)
	ds, err := loadDataset(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Task != "Explain recursion" {
		t.Errorf("task = %q", ds.Task)
	}
	if len(ds.Prompts) != 2 {
		t.Errorf("prompts = %d, want 2", len(ds.Prompts))
	}
	if ds.Prompts[1].System != "Be concise." {
		t.Errorf("system = %q", ds.Prompts[1].System)
	}
}

func TestLoadDataset_JSONL_WithHeader(t *testing.T) {
	raw := []byte(`{"task":"Write a haiku","params":["creativity","syllables"]}
{"id":"p1","user":"Haiku about Go channels"}
{"id":"p2","user":"Haiku about recursion"}
`)
	ds, err := loadDataset(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Task != "Write a haiku" {
		t.Errorf("task = %q", ds.Task)
	}
	if len(ds.Params) != 2 {
		t.Errorf("params = %v, want 2", ds.Params)
	}
	if len(ds.Prompts) != 2 {
		t.Errorf("prompts = %d, want 2", len(ds.Prompts))
	}
}

func TestLoadDataset_JSONL_WithoutHeader(t *testing.T) {
	raw := []byte(`{"id":"p1","user":"First prompt"}
{"id":"p2","user":"Second prompt"}
`)
	ds, err := loadDataset(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Prompts) != 2 {
		t.Errorf("prompts = %d, want 2", len(ds.Prompts))
	}
}

func TestLoadDataset_Empty_Error(t *testing.T) {
	if _, err := loadDataset([]byte("\n\n")); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestLoadDataset_InvalidJSON_Error(t *testing.T) {
	if _, err := loadDataset([]byte("{not valid json}")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadDataset_JSONL_InvalidPromptLine_Error(t *testing.T) {
	raw := []byte(`{"task":"test"}
{bad json}
`)
	if _, err := loadDataset(raw); err == nil {
		t.Error("expected error for invalid JSONL prompt line")
	}
}
