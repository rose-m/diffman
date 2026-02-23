package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName  = "lediff"
	configFileName = "config.json"
)

type AppConfig struct {
	LeaderCommands map[string]string `json:"leader_commands"`
}

func Load() (AppConfig, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return AppConfig{}, "", err
	}
	cfg, err := LoadFromPath(path)
	return cfg, path, err
}

func LoadFromPath(path string) (AppConfig, error) {
	cfg := AppConfig{
		LeaderCommands: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return AppConfig{}, err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.LeaderCommands == nil {
		cfg.LeaderCommands = make(map[string]string)
	}

	normalized := make(map[string]string, len(cfg.LeaderCommands))
	for k, v := range cfg.LeaderCommands {
		key := strings.TrimSpace(k)
		cmd := strings.TrimSpace(v)
		if len([]rune(key)) != 1 {
			return AppConfig{}, fmt.Errorf("leader command key %q must be a single character", k)
		}
		if key == " " {
			return AppConfig{}, fmt.Errorf("leader command key cannot be space")
		}
		if cmd == "" {
			return AppConfig{}, fmt.Errorf("leader command for key %q is empty", key)
		}
		normalized[key] = cmd
	}
	cfg.LeaderCommands = normalized

	return cfg, nil
}

func DefaultPath() (string, error) {
	home, err := configHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

func configHome() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return xdg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}
