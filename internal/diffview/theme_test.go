package diffview

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDarkFromColorFGBG(t *testing.T) {
	tests := []struct {
		name  string
		value string
		dark  bool
		ok    bool
	}{
		{name: "dark background", value: "15;0", dark: true, ok: true},
		{name: "light background", value: "0;15", dark: false, ok: true},
		{name: "with extra fields", value: "0;15;0", dark: true, ok: true},
		{name: "empty", value: "", dark: false, ok: false},
		{name: "non numeric", value: "foo;bar", dark: false, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dark, ok := darkFromColorFGBG(tt.value)
			if ok != tt.ok {
				t.Fatalf("darkFromColorFGBG(%q) ok=%v want %v", tt.value, ok, tt.ok)
			}
			if dark != tt.dark {
				t.Fatalf("darkFromColorFGBG(%q) dark=%v want %v", tt.value, dark, tt.dark)
			}
		})
	}
}

func TestInitializeThemeExplicitModes(t *testing.T) {
	defer applyDarkTheme()

	InitializeTheme("light")
	if cursorRowBg != lipgloss.Color("254") {
		t.Fatalf("light theme did not set expected cursor row color")
	}

	InitializeTheme("dark")
	if cursorRowBg != lipgloss.Color("236") {
		t.Fatalf("dark theme did not set expected cursor row color")
	}
}
