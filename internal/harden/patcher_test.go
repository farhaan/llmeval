package harden_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farhaan/llmeval/internal/harden"
)

func writeTempPrompt(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApplyPatch_NoOps(t *testing.T) {
	path := writeTempPrompt(t, "original content")
	if err := harden.ApplyPatch(path, nil); err != nil {
		t.Errorf("ApplyPatch with no ops returned error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original content" {
		t.Error("file should be unchanged with no ops")
	}
}

func TestApplyPatch_SingleOp(t *testing.T) {
	path := writeTempPrompt(t, "You are a helpful assistant. Be concise.")
	ops := []harden.PatchOp{
		{Find: "Be concise.", Replace: "Be concise and accurate."},
	}
	if err := harden.ApplyPatch(path, ops); err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "You are a helpful assistant. Be concise and accurate." {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestApplyPatch_MultipleOps(t *testing.T) {
	path := writeTempPrompt(t, "Hello world. Goodbye world.")
	ops := []harden.PatchOp{
		{Find: "Hello", Replace: "Hi"},
		{Find: "Goodbye", Replace: "Farewell"},
	}
	if err := harden.ApplyPatch(path, ops); err != nil {
		t.Fatalf("ApplyPatch error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "Hi world. Farewell world." {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestApplyPatch_FindNotFound(t *testing.T) {
	path := writeTempPrompt(t, "some content")
	ops := []harden.PatchOp{
		{Find: "nonexistent string", Replace: "replacement"},
	}
	err := harden.ApplyPatch(path, ops)
	if err == nil {
		t.Error("expected error when find string not in file, got nil")
	}
}

func TestApplyPatch_FileNotFound(t *testing.T) {
	err := harden.ApplyPatch("/nonexistent/path/prompt.md", []harden.PatchOp{
		{Find: "x", Replace: "y"},
	})
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestApplyPatch_ReplacesOnlyFirst(t *testing.T) {
	// strings.Replace with n=1 replaces only the first occurrence.
	path := writeTempPrompt(t, "foo foo foo")
	ops := []harden.PatchOp{
		{Find: "foo", Replace: "bar"},
	}
	if err := harden.ApplyPatch(path, ops); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "bar foo foo" {
		t.Errorf("expected only first occurrence replaced, got %q", string(data))
	}
}
