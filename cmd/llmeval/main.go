package main

import (
	"fmt"
	"os"

	"github.com/farhaan/llmeval/internal/cli"
	"github.com/farhaan/llmeval/internal/version"
)

const synopsis = `llmeval — LLM response collector for Claude Code evaluation

Usage:
  llmeval collect [flags]   Collect model responses for a dataset
  llmeval harden [flags]    Adversarially harden prompts via red-team loop
  llmeval providers         List configured providers and API key status
  llmeval version           Print version

Run 'llmeval <command> --help' for flags specific to each command.

Examples:
  llmeval collect --dataset /tmp/dataset.json --models "groq/llama-3.3-70b-versatile,openai/gpt-4o"
  llmeval collect --dataset /tmp/dataset.json --task-id explain-recursion --output /tmp/raw.json
  llmeval harden --all
  llmeval harden --prompt priv/prompts/product_bot.md
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, synopsis)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "collect":
		cli.Collect(os.Args[2:])
	case "harden":
		cli.Harden(os.Args[2:])
	case "providers":
		cli.Providers(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("llmeval", version.GetVersion())
	case "help", "--help", "-h":
		fmt.Print(synopsis)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], synopsis)
		os.Exit(1)
	}
}
