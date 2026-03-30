package harden

import (
	"fmt"
	"os"
	"strings"
)

// ApplyPatch modifies the prompt file in-place so the file remains the canonical prompt state
// across iterations. Strict find-or-error prevents silent no-ops — a patch that can't be located
// likely means a prior iteration already replaced that text, which signals a logic error in the
// AI-generated patch rather than a recoverable condition.
func ApplyPatch(promptPath string, ops []PatchOp) error {
	if len(ops) == 0 {
		return nil
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("read prompt for patch: %w", err)
	}

	content := string(data)
	for _, op := range ops {
		if !strings.Contains(content, op.Find) {
			return fmt.Errorf("patch find string not found in prompt: %q", op.Find)
		}
		content = strings.Replace(content, op.Find, op.Replace, 1)
	}

	if err := os.WriteFile(promptPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write patched prompt: %w", err)
	}
	return nil
}
