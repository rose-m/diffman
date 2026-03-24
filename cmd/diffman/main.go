package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"diffman/internal/app"
)

func main() {
	var pr string
	flag.StringVar(&pr, "pr", "", "GitHub pull request number or URL")
	flag.Parse()

	model, err := app.NewModelWithOptions(app.Options{PR: pr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize app: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}
