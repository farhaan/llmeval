package dotenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farhaan/llmeval/internal/dotenv"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoad_BasicKeyValue(t *testing.T) {
	path := writeFile(t, "FOO=bar\nBAZ=qux\n")
	_ = os.Unsetenv("FOO")
	_ = os.Unsetenv("BAZ")

	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("FOO"); got != "bar" {
		t.Errorf("FOO = %q, want %q", got, "bar")
	}
	if got := os.Getenv("BAZ"); got != "qux" {
		t.Errorf("BAZ = %q, want %q", got, "qux")
	}
}

func TestLoad_QuotedValues(t *testing.T) {
	path := writeFile(t, `DOUBLE="hello world"
SINGLE='hi there'
`)
	_ = os.Unsetenv("DOUBLE")
	_ = os.Unsetenv("SINGLE")

	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DOUBLE"); got != "hello world" {
		t.Errorf("DOUBLE = %q, want %q", got, "hello world")
	}
	if got := os.Getenv("SINGLE"); got != "hi there" {
		t.Errorf("SINGLE = %q, want %q", got, "hi there")
	}
}

func TestLoad_ShellEnvWins(t *testing.T) {
	path := writeFile(t, "EXISTING=from_file\n")
	t.Setenv("EXISTING", "from_shell")

	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("EXISTING"); got != "from_shell" {
		t.Errorf("EXISTING = %q, want shell value %q", got, "from_shell")
	}
}

func TestLoad_SkipsCommentsAndBlankLines(t *testing.T) {
	path := writeFile(t, `
# this is a comment
KEY=value

# another comment
OTHER=yes
`)
	_ = os.Unsetenv("KEY")
	_ = os.Unsetenv("OTHER")

	if err := dotenv.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("KEY"); got != "value" {
		t.Errorf("KEY = %q, want %q", got, "value")
	}
	if got := os.Getenv("OTHER"); got != "yes" {
		t.Errorf("OTHER = %q, want %q", got, "yes")
	}
}

func TestLoad_MissingFileIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.env")
	if err := dotenv.Load(path); err != nil {
		t.Errorf("Load on missing file returned error: %v", err)
	}
}
