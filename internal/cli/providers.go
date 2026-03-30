package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/farhaan/llmeval/internal/config"
	"github.com/farhaan/llmeval/internal/dotenv"
)

// Providers gives users a fast pre-flight key hygiene check before starting a run,
// catching absent keys here rather than failing partway through a batch of API calls.
func Providers(args []string) {
	fs := flag.NewFlagSet("providers", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: llmeval providers [flags]\n\nFlags:")
		fs.PrintDefaults()
	}
	configPath := fs.String("config", "", "Path to llmeval.yaml (optional)")
	_ = fs.Parse(args)

	_ = dotenv.Load(".env")
	_ = dotenv.Load(".llmeval/.env")

	cp := *configPath
	if cp == "" {
		if _, err := os.Stat("llmeval.yaml"); err == nil {
			cp = "llmeval.yaml"
		}
	}
	cfg, err := config.Load(cp)
	if err != nil {
		fatalf("load config: %v", err)
	}

	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)

	const (
		colProvider = 16
		colEnvVar   = 24
	)

	line := strings.Repeat("─", colProvider+colEnvVar+10)
	fmt.Println(line)
	fmt.Printf("%-*s  %-*s  %s\n", colProvider, "PROVIDER", colEnvVar, "ENV VAR", "STATUS")
	fmt.Println(line)

	for _, name := range names {
		pc := cfg.Providers[name]
		envVar := pc.EnvVar(name)
		status := "not set ✗"
		if os.Getenv(envVar) != "" {
			status = "set ✓"
		}
		fmt.Printf("%-*s  %-*s  %s\n", colProvider, name, colEnvVar, envVar, status)
	}

	fmt.Println(line)
	fmt.Println()
	fmt.Println("Set missing keys via shell export or a .env file in your project root.")
}
