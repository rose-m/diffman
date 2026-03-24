package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"

	"diffman/internal/comments"
	"diffman/internal/diffview"
	"diffman/internal/git"
	"diffman/internal/githubpr"
)

type mockPRService struct {
	diffCalls   int
	submitCalls int
	patches     map[string]string
}

func (m *mockPRService) ResolvePR(context.Context, string, string) (githubpr.Context, error) {
	return githubpr.Context{}, nil
}

func (m *mockPRService) ListOpenPRs(context.Context, string) ([]githubpr.Summary, error) {
	return nil, nil
}

func (m *mockPRService) ListFiles(context.Context, githubpr.Context) ([]git.FileItem, error) {
	return nil, nil
}

func (m *mockPRService) Diff(_ context.Context, _ githubpr.Context, path string) (string, error) {
	m.diffCalls++
	return m.patches[path], nil
}

func (m *mockPRService) SubmitReviewComments(context.Context, githubpr.Context, string, string, []comments.Comment) error {
	m.submitCalls++
	return nil
}

func TestPRModeDiffCachingAvoidsSecondFetch(t *testing.T) {
	service := &mockPRService{
		patches: map[string]string{
			"a.go": "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}

	m := Model{
		reviewMode: reviewModePR,
		prSvc:      service,
		prCtx:      &githubpr.Context{Owner: "acme", Repo: "widgets", Number: 42},
		prDiffs:    make(map[string]prDiffCacheEntry),
		oldView:    viewport.New(80, 20),
		newView:    viewport.New(80, 20),
	}

	first := m.loadDiffCmd("a.go")().(diffLoadedMsg)
	if service.diffCalls != 1 {
		t.Fatalf("expected first load to fetch from API once, got %d", service.diffCalls)
	}

	nextModel, _ := m.Update(first)
	m2 := nextModel.(Model)

	second := m2.loadDiffCmd("a.go")().(diffLoadedMsg)
	if service.diffCalls != 1 {
		t.Fatalf("expected second load to use cache, got %d API calls", service.diffCalls)
	}
	if second.err != nil {
		t.Fatalf("cached load returned error: %v", second.err)
	}
	if second.empty || len(second.rows) == 0 {
		t.Fatalf("cached load should return parsed rows")
	}
}

func TestPRModeStaleCheckUsesDiffCacheWhenAvailable(t *testing.T) {
	service := &mockPRService{}

	m := Model{
		reviewMode: reviewModePR,
		prSvc:      service,
		prCtx:      &githubpr.Context{Owner: "acme", Repo: "widgets", Number: 42},
		prDiffs: map[string]prDiffCacheEntry{
			"a.go": {
				rows: []diffview.DiffRow{
					{Kind: diffview.RowChange, Path: "a.go", OldLine: intPtr(1), NewLine: intPtr(1), OldText: "old", NewText: "new"},
				},
			},
		},
	}

	items := []git.FileItem{{Path: "a.go", Status: "M."}}
	commentMap := map[string]comments.Comment{
		comments.AnchorKey("a.go", comments.SideNew, 1): {
			Path: "a.go",
			Side: comments.SideNew,
			Line: 1,
			Body: "test",
		},
	}

	msg := m.loadCommentStaleCmd(items, commentMap, git.DiffModeAll)().(commentStaleLoadedMsg)
	if msg.err != nil {
		t.Fatalf("expected no error, got %v", msg.err)
	}
	if service.diffCalls != 0 {
		t.Fatalf("expected stale check to use cache, got %d API calls", service.diffCalls)
	}
	if stale := msg.stale[comments.AnchorKey("a.go", comments.SideNew, 1)]; stale {
		t.Fatalf("expected cached-line comment to be non-stale")
	}
}

func TestSubmitPRCommentsRemovesSubmittedDrafts(t *testing.T) {
	service := &mockPRService{}
	key := comments.AnchorKey("a.go", comments.SideNew, 1)
	draft := comments.Comment{Path: "a.go", Side: comments.SideNew, Line: 1, Body: "ship it"}

	m := Model{
		reviewMode:   reviewModePR,
		prSvc:        service,
		prCtx:        &githubpr.Context{Owner: "acme", Repo: "widgets", Number: 42},
		commentStore: comments.NewStore(t.TempDir()),
		comments: map[string]comments.Comment{
			key: draft,
		},
		commentStale: map[string]bool{key: false},
		oldView:      viewport.New(80, 20),
		newView:      viewport.New(80, 20),
	}

	cmd := m.submitReviewCmd([]comments.Comment{draft}, "body", reviewEventComment)
	msg := cmd().(submitReviewResultMsg)
	nextModel, _ := m.Update(msg)
	m2 := nextModel.(Model)

	if service.submitCalls != 1 {
		t.Fatalf("expected one submit call, got %d", service.submitCalls)
	}
	if len(m2.comments) != 0 {
		t.Fatalf("expected submitted comment to be removed locally")
	}
	if len(m2.commentStale) != 0 {
		t.Fatalf("expected stale map entry removed with submitted comment")
	}
}
