package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"lediff/internal/util"
)

type DiffMode int

const (
	DiffModeAll DiffMode = iota
	DiffModeUnstaged
	DiffModeStaged
)

func (m DiffMode) String() string {
	switch m {
	case DiffModeAll:
		return "all"
	case DiffModeUnstaged:
		return "unstaged"
	case DiffModeStaged:
		return "staged"
	default:
		return "unknown"
	}
}

type DiffService interface {
	Diff(ctx context.Context, cwd, path string, mode DiffMode) (string, error)
}

type diffService struct{}

func NewDiffService() DiffService {
	return diffService{}
}

func (diffService) Diff(ctx context.Context, cwd, path string, mode DiffMode) (string, error) {
	args := []string{"diff", "-U3", "--", path}
	switch mode {
	case DiffModeAll:
		args = []string{"diff", "HEAD", "-U3", "--", path}
	case DiffModeStaged:
		args = []string{"diff", "--cached", "-U3", "--", path}
	case DiffModeUnstaged:
		args = []string{"diff", "-U3", "--", path}
	}

	out, err := util.Run(ctx, cwd, "git", args...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
	}

	if mode == DiffModeStaged {
		return "", nil
	}

	// Fallback for untracked paths; --no-index returns exit code 1 when diff exists.
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--", "/dev/null", path)
	if cwd != "" {
		cmd.Dir = cwd
	}
	noIndexOut, noIndexErr := cmd.CombinedOutput()
	if noIndexErr == nil {
		return string(noIndexOut), nil
	}

	var exitErr *exec.ExitError
	if errors.As(noIndexErr, &exitErr) && exitErr.ExitCode() == 1 {
		return string(noIndexOut), nil
	}

	// If the path isn't an untracked file (or can't be diffed this way), treat as no diff.
	return "", nil
}
