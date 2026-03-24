package githubpr

import (
	"context"

	gitint "diffman/internal/git"
)

type Context struct {
	Owner   string
	Repo    string
	Number  int
	Title   string
	URL     string
	HeadSHA string
	HeadRef string
	BaseRef string
}

type Service interface {
	ResolvePR(ctx context.Context, cwd, input string) (Context, error)
	ListFiles(ctx context.Context, pr Context) ([]gitint.FileItem, error)
	Diff(ctx context.Context, pr Context, path string) (string, error)
}

func NewService() Service {
	return ghService{}
}
