package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"diffman/internal/comments"
	"diffman/internal/git"
	"diffman/internal/githubpr"
)

type pickerPRService struct {
	resolved githubpr.Context
}

func (s *pickerPRService) ListOpenPRs(context.Context, string) ([]githubpr.Summary, error) {
	return []githubpr.Summary{{Number: 42, Title: "Test PR"}}, nil
}

func (s *pickerPRService) ResolvePR(context.Context, string, string) (githubpr.Context, error) {
	return s.resolved, nil
}

func (s *pickerPRService) ListFiles(context.Context, githubpr.Context) ([]git.FileItem, error) {
	return nil, nil
}

func (s *pickerPRService) Diff(context.Context, githubpr.Context, string) (string, error) {
	return "", nil
}

func (s *pickerPRService) SubmitReviewComments(context.Context, githubpr.Context, string, string, []comments.Comment) error {
	return nil
}

func TestQuitFromPRReviewReturnsToPicker(t *testing.T) {
	m := Model{
		reviewMode: reviewModePR,
		prCtx:      &githubpr.Context{Owner: "acme", Repo: "widgets", Number: 7},
		prPicker:   false,
		prItems:    []githubpr.Summary{{Number: 7, Title: "A"}},
		prDiffs:    map[string]prDiffCacheEntry{"a.go": {}},
		keys:       defaultKeyMap(),
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 := next.(Model)

	if !m2.prPicker {
		t.Fatalf("expected q to return to PR picker")
	}
	if m2.prCtx != nil {
		t.Fatalf("expected active PR context to be cleared when returning to picker")
	}
}

func TestEnterInPRPickerResolvesAndEntersReview(t *testing.T) {
	svc := &pickerPRService{resolved: githubpr.Context{Owner: "acme", Repo: "widgets", Number: 42}}
	m := Model{
		reviewMode: reviewModePR,
		prPicker:   true,
		prSvc:      svc,
		cwd:        "/tmp",
		prItems:    []githubpr.Summary{{Number: 42, Title: "Test PR"}},
		keys:       defaultKeyMap(),
	}

	modelAfterEnter, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected enter to start PR resolve command")
	}

	resolvedMsg := cmd().(prResolvedMsg)
	next, _ := modelAfterEnter.(Model).Update(resolvedMsg)
	m2 := next.(Model)

	if m2.prPicker {
		t.Fatalf("expected picker to close after PR resolve")
	}
	if m2.prCtx == nil || m2.prCtx.Number != 42 {
		t.Fatalf("expected resolved PR context to be active")
	}
}
