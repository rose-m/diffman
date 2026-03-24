package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"diffman/internal/app"
)

func main() {
	var prMode bool
	var prRef string
	flag.BoolVar(&prMode, "pr", false, "Launch in GitHub PR mode (open PR picker)")
	flag.StringVar(&prRef, "pr-ref", "", "GitHub pull request number or URL")
	flag.Parse()
	if prRef != "" {
		prMode = true
	}

	model, err := app.NewModelWithOptions(app.Options{PR: prRef, PRPicker: prMode && prRef == ""})
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
