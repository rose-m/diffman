package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"diffman/internal/app"
)

func main() {
	model, err := app.NewModel()
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
