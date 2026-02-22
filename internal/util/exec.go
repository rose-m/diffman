package util

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func Run(ctx context.Context, cwd string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}
