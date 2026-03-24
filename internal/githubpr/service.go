package githubpr

import (
	"context"

	"diffman/internal/comments"
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

type Summary struct {
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
	ListOpenPRs(ctx context.Context, cwd string) ([]Summary, error)
	ResolvePR(ctx context.Context, cwd, input string) (Context, error)
	ListFiles(ctx context.Context, pr Context) ([]gitint.FileItem, error)
	Diff(ctx context.Context, pr Context, path string) (string, error)
	SubmitReviewComments(ctx context.Context, pr Context, body, event string, draft []comments.Comment) error
}

func NewService() Service {
	return ghService{}
}
