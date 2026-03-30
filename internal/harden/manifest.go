package harden

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadManifest is the single entry point for manifest YAML so all callers work with typed
// PromptEntry values rather than parsing YAML inline — centralising the decode also means
// format changes only need to be fixed in one place.
func LoadManifest(path string) ([]PromptEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m PromptManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m.Prompts, nil
}

// FillFixtures lets one prompt file serve multiple scenarios by substituting {{key}} placeholders —
// avoiding duplicate near-identical prompt files for different locales, user roles, or environments.
func FillFixtures(template string, fixtures map[string]string) string {
	result := template
	for k, v := range fixtures {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
