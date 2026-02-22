package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"lediff/internal/util"
)

type DiffService interface {
	AllChangesDiff(ctx context.Context, cwd, path string) (string, error)
}

type diffService struct{}

func NewDiffService() DiffService {
	return diffService{}
}

func (diffService) AllChangesDiff(ctx context.Context, cwd, path string) (string, error) {
	out, err := util.Run(ctx, cwd, "git", "diff", "HEAD", "-U3", "--", path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
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
