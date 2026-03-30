package history

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/farhaan/llmeval/internal/cache"
)

var headers = []string{
	"timestamp", "hash", "task_id", "task", "models", "dataset_size", "concurrency", "config", "forced",
}

func FilePath() string {
	return filepath.Join(cache.BaseDir(), "history.csv")
}

// Entry is one row in the history file, written after every successful collect run.
type Entry struct {
	Hash        string
	TaskID      string // empty if --task-id was not used
	Task        string // task description
	Models      []string
	DatasetSize int
	Concurrency int
	Config      string // path used, or "" for built-in defaults
	Forced      bool   // true when --force bypassed the cache
}

// Append records each run in a lightweight audit trail. Write errors are silently ignored
// because history is best-effort — losing a log entry is less harmful than crashing the
// collect run that produced the real output.
func Append(e Entry) {
	path := FilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	needsHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	if needsHeader {
		_ = w.Write(headers)
	}
	// Truncate the task description to limit PII surface area in the history file.
	// The full description is available in the cached RawOutput JSON.
	const maxTaskLen = 100
	task := e.Task
	if len(task) > maxTaskLen {
		task = task[:maxTaskLen] + "…"
	}

	forced := "false"
	if e.Forced {
		forced = "true"
	}
	_ = w.Write([]string{
		time.Now().UTC().Format(time.RFC3339),
		e.Hash,
		e.TaskID,
		task,
		strings.Join(e.Models, "|"),
		strconv.Itoa(e.DatasetSize),
		strconv.Itoa(e.Concurrency),
		e.Config,
		forced,
	})
	w.Flush()
}
