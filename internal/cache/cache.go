package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func BaseDir() string {
	return ".llmeval"
}

func Dir() string {
	return filepath.Join(BaseDir(), "cache")
}

func FilePath(hash string) string {
	return filepath.Join(Dir(), hash+".json")
}

// Load reads a cached result. Returns (data, true) on hit with valid JSON.
// On a corrupt or partial file it removes the entry and returns (nil, false),
// causing the caller to re-run and overwrite the bad cache entry.
func Load(hash string) ([]byte, bool) {
	path := FilePath(hash)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !json.Valid(data) {
		_ = os.Remove(path)
		return nil, false
	}
	return data, true
}

// Save writes data to the cache atomically via a temp-file rename.
// Concurrent writers for the same hash are safe: the last rename wins
// and readers never see a partially-written file.
// Silently ignores write errors.
func Save(hash string, data []byte) {
	path := FilePath(hash)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + fmt.Sprintf(".tmp%d", os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
