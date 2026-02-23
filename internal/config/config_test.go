package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromPathMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if len(cfg.LeaderCommands) != 0 {
		t.Fatalf("expected empty commands, got %d", len(cfg.LeaderCommands))
	}
}

func TestLoadFromPathParsesLeaderCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"leader_commands":{"g":"lazygit","t":"tmux attach"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if got, ok := cfg.LeaderCommands["g"]; !ok || got != "lazygit" {
		t.Fatalf("expected g=lazygit, got %q (exists=%v)", got, ok)
	}
	if got, ok := cfg.LeaderCommands["t"]; !ok || got != "tmux attach" {
		t.Fatalf("expected t=tmux attach, got %q (exists=%v)", got, ok)
	}
}

func TestLoadFromPathRejectsInvalidKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"leader_commands":{"gg":"lazygit"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadFromPath(path); err == nil {
		t.Fatalf("expected error for invalid key")
	}
}

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	want := filepath.Join(xdg, "diffman", "config.json")
	if got != want {
		t.Fatalf("DefaultPath()=%q want %q", got, want)
	}
}
