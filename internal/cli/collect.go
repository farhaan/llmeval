package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/farhaan/llmeval/internal/cache"
	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dataset"
	"github.com/farhaan/llmeval/internal/dotenv"
	"github.com/farhaan/llmeval/internal/hasher"
	"github.com/farhaan/llmeval/internal/history"
	"github.com/farhaan/llmeval/internal/providers"
	"github.com/farhaan/llmeval/internal/runner"
)

// Collect is the CLI entry point for the collect subcommand.
// Accepts dataset as JSON (.json) or JSONL (.jsonl).
//
// Flag precedence (highest to lowest):
//
//	CLI flag  >  task config (--task-id)  >  global config  >  built-in defaults
func Collect(args []string) {
	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: llmeval collect [flags]\n\nFlags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nFlag precedence: CLI flag > task config (--task-id) > global config > built-in defaults")
	}
	datasetPath := fs.String("dataset", "", "Path to dataset JSON or JSONL file (required)")
	modelsStr := fs.String("models", "", "Comma-separated model IDs. Defaults to task models if --task-id is set.")
	taskID := fs.String("task-id", "", "Named task from llmeval.yaml tasks section; sets model/concurrency defaults")
	configPath := fs.String("config", "", "Path to llmeval.yaml (optional; uses ./llmeval.yaml or built-in defaults)")
	outputPath := fs.String("output", "", "Write raw output JSON to this file (default: stdout)")
	concurrency := fs.Int("concurrency", 0, "Max parallel API calls (default: from task/config or 5)")
	force := fs.Bool("force", false, "Ignore cached result and re-run")
	envFile := fs.String("env-file", "", "Path to .env file (default: .env and .llmeval/.env in CWD)")
	_ = fs.Parse(args)

	if *datasetPath == "" {
		fmt.Fprintln(os.Stderr, "error: --dataset is required")
		fs.Usage()
		os.Exit(1)
	}

	loadEnvFiles(*envFile)

	cp := resolveCollectConfigPath(*configPath)
	cfg, err := config.Load(cp)
	if err != nil {
		fatalf("load config: %v", err)
	}

	task := applyTaskDefaults(cfg, *taskID)

	models := resolveModels(*modelsStr, task, fs)

	if *concurrency > 0 {
		cfg.Concurrency = *concurrency
	}

	ds := readAndValidateDataset(*datasetPath, cfg)

	hash := hasher.Compute(models, ds, cfg.ModelConfig)

	if !*force {
		if cached, ok := cache.Load(hash); ok {
			fmt.Fprintln(os.Stderr, "cache hit:", hash)
			writeOutput(*outputPath, cached)
			return
		}
	}

	reg, err := providers.NewRegistry(cfg)
	if err != nil {
		fatalf("build registry: %v", err)
	}

	preflightCheck(reg, models)

	fmt.Fprintf(os.Stderr, "running %d model(s) × %d prompt(s) (concurrency %d)...\n",
		len(models), len(ds.Prompts), cfg.Concurrency)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	responses := runner.Run(ctx, reg, models, ds, cfg.ModelConfig, cfg.Concurrency)

	warnFailedResponses(responses)

	out := dataset.RawOutput{
		Hash:      hash,
		Dataset:   ds,
		Models:    models,
		Responses: responses,
	}
	data, _ := json.MarshalIndent(out, "", "  ")

	cache.Save(hash, data)
	history.Append(history.Entry{
		Hash:        hash,
		TaskID:      *taskID,
		Task:        ds.Task,
		Models:      models,
		DatasetSize: len(ds.Prompts),
		Concurrency: cfg.Concurrency,
		Config:      cp,
		Forced:      *force,
	})

	writeOutput(*outputPath, data)
}

func loadEnvFiles(envFile string) {
	if envFile != "" {
		_ = dotenv.Load(envFile)
	} else {
		_ = dotenv.Load(".env")
		_ = dotenv.Load(".llmeval/.env")
	}
}

func resolveCollectConfigPath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if _, err := os.Stat("llmeval.yaml"); err == nil {
		return "llmeval.yaml"
	}
	return ""
}

func applyTaskDefaults(cfg *config.Config, taskID string) config.TaskConfig {
	if taskID == "" {
		return config.TaskConfig{}
	}
	t, ok := cfg.Tasks[taskID]
	if !ok {
		fatalf("task %q not found in config — available tasks: %s",
			taskID, availableTasks(cfg))
	}
	if t.Concurrency > 0 {
		cfg.Concurrency = t.Concurrency
	}
	if t.DatasetSize > 0 {
		cfg.DatasetSize = t.DatasetSize
	}
	return t
}

func resolveModels(modelsStr string, task config.TaskConfig, fs *flag.FlagSet) []string {
	models := splitTrim(modelsStr)
	if len(models) == 0 && len(task.Models) > 0 {
		models = task.Models
	}
	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "error: --models is required (or define models in the task config with --task-id)")
		fs.Usage()
		os.Exit(1)
	}
	return models
}

func readAndValidateDataset(datasetPath string, cfg *config.Config) dataset.Dataset {
	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		fatalf("read dataset: %v", err)
	}
	ds, err := loadDataset(raw)
	if err != nil {
		fatalf("parse dataset %s: %v", datasetPath, err)
	}
	if cfg.DatasetSize > 0 && len(ds.Prompts) != cfg.DatasetSize {
		fmt.Fprintf(os.Stderr, "warning: dataset has %d prompt(s) but dataset_size is configured as %d — running with actual count\n",
			len(ds.Prompts), cfg.DatasetSize)
	}
	if dups := duplicatePromptIDs(ds); len(dups) > 0 {
		fatalf("dataset has duplicate prompt IDs: %s", strings.Join(dups, ", "))
	}
	for _, p := range ds.Prompts {
		if strings.TrimSpace(p.User) == "" {
			fatalf("prompt %q has an empty user field", p.ID)
		}
	}
	return ds
}

func preflightCheck(reg interface {
	ValidateModels([]string) []error
	MissingKeys() []string
}, models []string) {
	if errs := reg.ValidateModels(models); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "error:", e)
		}
		os.Exit(1)
	}
	warnMissingKeys(reg.MissingKeys(), models)
}

func warnMissingKeys(missing []string, models []string) {
	for _, name := range missing {
		for _, m := range models {
			if len(m) >= len(name) && m[:len(name)] == name {
				fmt.Fprintf(os.Stderr, "warning: %s_API_KEY is not set — calls to %s will fail with 401\n",
					strings.ToUpper(name), name)
				break
			}
		}
	}
}

func warnFailedResponses(responses []dataset.Response) {
	var failCount int
	for _, r := range responses {
		if r.Error != "" {
			failCount++
		}
	}
	if failCount > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d/%d responses failed — check the 'error' fields in the output\n",
			failCount, len(responses))
	}
}

// loadDataset accepts both JSON and JSONL so collect can consume output from other tools
// (jq pipelines, dataset generators) without requiring format conversion.
func loadDataset(raw []byte) (dataset.Dataset, error) {
	var ds dataset.Dataset
	if err := json.Unmarshal(raw, &ds); err == nil {
		if len(ds.Prompts) == 0 {
			return dataset.Dataset{}, fmt.Errorf("no prompts found")
		}
		return ds, nil
	}
	return loadJSONLDataset(raw)
}

func loadJSONLDataset(raw []byte) (dataset.Dataset, error) {
	lines, err := scanNonEmptyLines(raw)
	if err != nil {
		return dataset.Dataset{}, err
	}
	if len(lines) == 0 {
		return dataset.Dataset{}, fmt.Errorf("file is empty")
	}

	var ds dataset.Dataset
	start := 0
	if isJSONLHeader(lines[0]) {
		parseJSONLHeader(&ds, lines[0])
		start = 1
	}

	for _, line := range lines[start:] {
		var p dataset.Prompt
		if err := json.Unmarshal(line, &p); err != nil {
			return dataset.Dataset{}, fmt.Errorf("invalid JSONL line %q: %w", line, err)
		}
		ds.Prompts = append(ds.Prompts, p)
	}

	if len(ds.Prompts) == 0 {
		return dataset.Dataset{}, fmt.Errorf("no prompts found")
	}
	return ds, nil
}

func scanNonEmptyLines(raw []byte) ([][]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	var lines [][]byte
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func isJSONLHeader(line []byte) bool {
	var header struct {
		Task   string   `json:"task"`
		Params []string `json:"params"`
	}
	var rawHeader map[string]json.RawMessage
	if err := json.Unmarshal(line, &header); err != nil {
		return false
	}
	if err := json.Unmarshal(line, &rawHeader); err != nil {
		return false
	}
	_, hasUser := rawHeader["user"]
	return !hasUser && (header.Task != "" || len(header.Params) > 0)
}

func parseJSONLHeader(ds *dataset.Dataset, line []byte) {
	var header struct {
		Task   string   `json:"task"`
		Params []string `json:"params"`
	}
	if err := json.Unmarshal(line, &header); err == nil {
		ds.Task = header.Task
		ds.Params = header.Params
	}
}

func duplicatePromptIDs(ds dataset.Dataset) []string {
	seen := make(map[string]bool, len(ds.Prompts))
	var dups []string
	for _, p := range ds.Prompts {
		if seen[p.ID] {
			dups = append(dups, p.ID)
		}
		seen[p.ID] = true
	}
	return dups
}

func availableTasks(cfg *config.Config) string {
	if len(cfg.Tasks) == 0 {
		return "(none defined)"
	}
	names := make([]string, 0, len(cfg.Tasks))
	for id := range cfg.Tasks {
		names = append(names, id)
	}
	return strings.Join(names, ", ")
}

func writeOutput(path string, data []byte) {
	if path == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fatalf("write output: %v", err)
		}
		fmt.Println()
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatalf("write output: %v", err)
	}
	fmt.Fprintln(os.Stderr, "wrote", path)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
