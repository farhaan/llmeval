package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/farhaan/llmeval/internal/cache"
)

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	data := []byte(`{"key":"value"}`)
	cache.Save("abc123", data)

	got, ok := cache.Load("abc123")
	if !ok {
		t.Fatal("Load returned false, want true")
	}
	if string(got) != string(data) {
		t.Errorf("Load = %q, want %q", got, data)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, ok := cache.Load("nonexistent")
	if ok {
		t.Error("Load returned true for missing file, want false")
	}
}

func TestLoad_CorruptJSON_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Write invalid JSON directly to the cache path.
	path := cache.FilePath("corrupt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, ok := cache.Load("corrupt")
	if ok {
		t.Error("Load returned true for corrupt JSON, want false")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupt cache file should have been removed")
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// The .llmeval/cache dir does not exist yet.
	cache.Save("newentry", []byte(`{}`))

	if _, err := os.Stat(cache.FilePath("newentry")); err != nil {
		t.Errorf("expected cache file to exist after Save: %v", err)
	}
}

func TestSave_AtomicWrite_NoTempFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	cache.Save("atomic", []byte(`{"x":1}`))

	// After Save completes there must be no leftover .tmp* file.
	cacheDir := cache.Dir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			t.Errorf("unexpected non-json file in cache dir after Save: %s", e.Name())
		}
	}
}

// chdir changes the working directory for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}
