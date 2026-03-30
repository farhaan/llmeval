package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/harden"
)

// Harden wires CLI flags to harden.Run. It also intercepts *AINeededError — when the loop
// needs Claude to perform an AI step it writes a request file and signals the skill via
// exit code 10, which the skill reads to drive the next step without re-entering main().
func Harden(args []string) {
	fs := flag.NewFlagSet("harden", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: llmeval harden [flags]\n\nFlags:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nFlag precedence: CLI flag > llmeval.yaml harden section > built-in defaults")
	}
	promptPath := fs.String("prompt", "", "Single prompt file to harden (must match path in manifest)")
	all := fs.Bool("all", false, "Harden all prompts in manifest")
	manifest := fs.String("manifest", "", "Manifest path override")
	target := fs.String("target", "", "Target model override (e.g. groq/llama-3.3-70b-versatile)")
	maxIter := fs.Int("max-iterations", 0, "Max iterations override (0 = use config, negative = unlimited)")
	probes := fs.Int("probes", 0, "Probes per iteration override (0 = use config)")
	resume := fs.Bool("resume", false, "Resume from last checkpoint")
	force := fs.Bool("force", false, "Wipe checkpoints and start fresh")
	verbose := fs.Bool("verbose", false, "Print per-probe details")
	configPath := fs.String("config", "", "Path to llmeval.yaml (optional; auto-detects ./llmeval.yaml)")
	envFile := fs.String("env-file", "", "Path to .env file (default: .env and .llmeval/.env in CWD)")
	confirmationRuns := fs.Int("confirmation-runs", 0, "Re-runs when findings==0 to confirm clean (0 = use config)")
	probeRuns := fs.Int("probe-runs", 0, "Runs per probe for noise reduction (0 = use config)")
	dryRun := fs.Bool("dry-run", false, "Print what would run without executing")
	language := fs.String("language", "", "BCP-47 language code for linguistic expert analysis (e.g. en, id, ja)")
	skipGolden := fs.Bool("skip-golden", false, "Skip golden validation after patching")
	cache := fs.Bool("cache", false, "Skip run if prompt hash is unchanged from last completed run; wipe and re-run if hash differs")
	fromIter := fs.Int("from-iter", -1, "Re-run from this iteration index (wipes that iter and all later ones)")
	_ = fs.Parse(args)

	loadEnvFiles(*envFile)

	cp := resolveConfigPath(*configPath)
	cfg, err := config.Load(cp)
	if err != nil {
		fatalf("load config: %v", err)
	}

	entries := loadManifestEntries(*manifest, cfg)
	targets := resolveHardenTargets(entries, *all, *promptPath, *manifest)

	runCfg := buildRunConfig(cfg, *target, *maxIter, *probes, *confirmationRuns, *probeRuns, *resume, *force, *dryRun, *skipGolden, *cache, *language, *fromIter)

	reports := runHardenTargets(targets, runCfg, *verbose)
	printFinalSummary(reports)
}

func loadManifestEntries(manifestFlag string, cfg *config.Config) []harden.PromptEntry {
	manifestPath := manifestFlag
	if manifestPath == "" {
		manifestPath = cfg.Harden.Manifest
	}
	if manifestPath == "" {
		fatalf("--manifest is required (or set harden.manifest in llmeval.yaml)")
	}
	entries, err := harden.LoadManifest(manifestPath)
	if err != nil {
		fatalf("load manifest: %v", err)
	}
	if len(entries) == 0 {
		fatalf("manifest %q contains no prompts", manifestPath)
	}
	return entries
}

func resolveHardenTargets(entries []harden.PromptEntry, all bool, promptPath, manifestPath string) []harden.PromptEntry {
	if all {
		return entries
	}
	if promptPath == "" {
		fatalf("--prompt <path> or --all is required")
	}
	for _, e := range entries {
		if e.Path == promptPath {
			return []harden.PromptEntry{e}
		}
	}
	fatalf("prompt %q not found in manifest %q", promptPath, manifestPath)
	return nil
}

func runHardenTargets(targets []harden.PromptEntry, runCfg harden.RunConfig, verbose bool) []*harden.Report {
	var reports []*harden.Report
	for _, entry := range targets {
		entryTargets := entry.Targets
		if len(entryTargets) == 0 {
			entryTargets = []string{runCfg.Target}
		}
		for _, t := range entryTargets {
			report := runHardenEntry(entry, t, entryTargets, runCfg, verbose)
			if report != nil {
				reports = append(reports, report)
			}
		}
	}
	return reports
}

func runHardenEntry(entry harden.PromptEntry, t string, entryTargets []string, runCfg harden.RunConfig, verbose bool) *harden.Report {
	entryCfg := runCfg
	entryCfg.Target = t
	if len(entryTargets) > 1 {
		entryCfg.TargetSuffix = harden.TargetToStem(t)
	}
	fmt.Fprintf(os.Stderr, "\n[%s] Starting hardening (target: %s, probes: %d)...\n",
		entry.Name, entryCfg.Target, entryCfg.Probes)
	report, err := harden.Run(context.Background(), entryCfg, entry, verbose)
	var aiErr *harden.AINeededError
	if errors.As(err, &aiErr) {
		fmt.Fprintf(os.Stderr, "[AI needed] %s\n", aiErr.RequestFile)
		os.Exit(10)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s/%s] ERROR: %v\n", entry.Name, t, err)
		return nil
	}
	printReport(report)
	return report
}

// buildRunConfig resolves CLI-flag-wins-over-yaml-wins-over-defaults precedence in one place.
// harden.Run receives a fully-resolved RunConfig and never needs to know where a value came from.
func buildRunConfig(cfg *config.Config, target string, maxIter, probes, confirmationRuns, probeRuns int, resume, force, dryRun, skipGolden, cache bool, language string, fromIter int) harden.RunConfig {
	h := cfg.Harden

	resolvedTarget := h.Target
	if target != "" {
		resolvedTarget = target
	}
	if resolvedTarget == "" {
		fatalf("target model is required: set harden.target in llmeval.yaml or use --target flag")
	}

	resolvedProbes := h.Probes
	if probes > 0 {
		resolvedProbes = probes
	}

	resolvedMaxIter := h.MaxIterations
	if maxIter != 0 {
		if maxIter < 0 {
			resolvedMaxIter = 0 // negative = unlimited
		} else {
			resolvedMaxIter = maxIter
		}
	}

	resolvedConfirmationRuns := h.ConfirmationRuns
	if confirmationRuns > 0 {
		resolvedConfirmationRuns = confirmationRuns
	}

	resolvedProbeRuns := h.ProbeRuns
	if probeRuns > 0 {
		resolvedProbeRuns = probeRuns
	}

	return harden.RunConfig{
		Target:           resolvedTarget,
		JudgeModel:       h.JudgeModel,
		Probes:           resolvedProbes,
		MaxIterations:    resolvedMaxIter,
		Until:            h.Until,
		RunDir:           h.RunDir,
		Concurrency:      h.Concurrency,
		Config:           cfg,
		Resume:           resume,
		Force:            force,
		ConfirmationRuns: resolvedConfirmationRuns,
		ProbeRuns:        resolvedProbeRuns,
		DryRun:           dryRun,
		Language:         language,
		SkipGolden:       skipGolden || h.SkipGolden,
		Cache:            cache || h.Cache,
		FromIter:         fromIter,
	}
}

// resolveConfigPath makes --config optional: auto-detecting llmeval.yaml in the working
// directory keeps the common case zero-friction while still allowing an explicit override.
func resolveConfigPath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if _, err := os.Stat("llmeval.yaml"); err == nil {
		return "llmeval.yaml"
	}
	return ""
}

func printReport(r *harden.Report) {
	status := r.Status
	switch status {
	case "clean":
		status = "CLEAN ✓"
	case "reverted":
		status = "REVERTED (golden failed)"
	case "stuck":
		status = fmt.Sprintf("STUCK (%d findings remain)", r.FinalFindings)
	}
	fmt.Fprintf(os.Stderr, "\n[%s] %s — %d iter(s), %d patch op(s), %d finding(s) remaining\n",
		r.Name, status, r.TotalIters, r.TotalPatches, r.FinalFindings)
}

func printFinalSummary(reports []*harden.Report) {
	if len(reports) == 0 {
		fmt.Fprintln(os.Stderr, "\nNo reports — all prompts failed or were skipped.")
		return
	}
	clean, stuck, reverted := 0, 0, 0
	for _, r := range reports {
		switch r.Status {
		case "clean":
			clean++
		case "stuck":
			stuck++
		case "reverted":
			reverted++
		}
	}
	fmt.Fprintf(os.Stderr, "\n── Harden Summary ──────────────────────────────────────\n")
	fmt.Fprintf(os.Stderr, "  Clean:    %d\n", clean)
	fmt.Fprintf(os.Stderr, "  Stuck:    %d\n", stuck)
	fmt.Fprintf(os.Stderr, "  Reverted: %d\n", reverted)
	fmt.Fprintf(os.Stderr, "  Total:    %d\n", len(reports))
	fmt.Fprintln(os.Stderr, "────────────────────────────────────────────────────────")
}
