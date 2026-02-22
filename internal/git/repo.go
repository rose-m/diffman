package git

import (
	"context"
	"strings"

	"lediff/internal/util"
)

func DiscoverRepoRoot(ctx context.Context, cwd string) (string, error) {
	out, err := util.Run(ctx, cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func DiscoverGitDir(ctx context.Context, cwd string) (string, error) {
	out, err := util.Run(ctx, cwd, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
