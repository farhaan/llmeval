package history_test

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/farhaan/llmeval/internal/history"
)

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

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV: %v", err)
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	return rows
}

func TestAppend_CreatesFileWithHeader(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	history.Append(history.Entry{
		Hash:        "abc",
		Task:        "test task",
		Models:      []string{"gpt-4"},
		DatasetSize: 5,
		Concurrency: 2,
	})

	rows := readCSV(t, history.FilePath())
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows (header + data), got %d", len(rows))
	}
	header := rows[0]
	if header[0] != "timestamp" {
		t.Errorf("first column = %q, want %q", header[0], "timestamp")
	}
	if header[1] != "hash" {
		t.Errorf("second column = %q, want %q", header[1], "hash")
	}
}

func TestAppend_WritesCorrectValues(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	history.Append(history.Entry{
		Hash:        "deadbeef",
		TaskID:      "task-001",
		Task:        "evaluate responses",
		Models:      []string{"gpt-4", "claude-3"},
		DatasetSize: 10,
		Concurrency: 4,
		Config:      "llmeval.yaml",
		Forced:      true,
	})

	rows := readCSV(t, history.FilePath())
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	data := rows[1]

	// hash
	if data[1] != "deadbeef" {
		t.Errorf("hash = %q, want deadbeef", data[1])
	}
	// task_id
	if data[2] != "task-001" {
		t.Errorf("task_id = %q, want task-001", data[2])
	}
	// models joined with |
	if data[4] != "gpt-4|claude-3" {
		t.Errorf("models = %q, want gpt-4|claude-3", data[4])
	}
	// dataset_size
	if data[5] != "10" {
		t.Errorf("dataset_size = %q, want 10", data[5])
	}
	// forced
	if data[8] != "true" {
		t.Errorf("forced = %q, want true", data[8])
	}
}

func TestAppend_AppendsTwoRows(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	history.Append(history.Entry{Hash: "hash1", Task: "first"})
	history.Append(history.Entry{Hash: "hash2", Task: "second"})

	rows := readCSV(t, history.FilePath())
	// header + 2 data rows
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[1][1] != "hash1" {
		t.Errorf("first data row hash = %q", rows[1][1])
	}
	if rows[2][1] != "hash2" {
		t.Errorf("second data row hash = %q", rows[2][1])
	}
}

func TestAppend_HeaderWrittenOnce(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	for i := 0; i < 3; i++ {
		history.Append(history.Entry{Hash: "h", Task: "t"})
	}

	rows := readCSV(t, history.FilePath())
	// Only 1 header, 3 data rows = 4 total
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows (1 header + 3 data), got %d", len(rows))
	}
	// Second row should be data, not another header
	if rows[1][0] == "timestamp" {
		t.Error("header was written more than once")
	}
}

func TestAppend_TaskTruncated(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	longTask := strings.Repeat("x", 200)
	history.Append(history.Entry{Hash: "h", Task: longTask})

	rows := readCSV(t, history.FilePath())
	if len(rows) < 2 {
		t.Fatal("no data row")
	}
	task := rows[1][3]
	if len([]rune(task)) > 105 { // 100 chars + "…" (multibyte)
		t.Errorf("task not truncated, len=%d", len(task))
	}
}

func TestAppend_ForcedFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	history.Append(history.Entry{Hash: "h", Forced: false})

	rows := readCSV(t, history.FilePath())
	if rows[1][8] != "false" {
		t.Errorf("forced = %q, want false", rows[1][8])
	}
}
