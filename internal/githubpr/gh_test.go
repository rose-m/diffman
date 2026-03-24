package githubpr

import (
	"strings"
	"testing"

	"diffman/internal/comments"
)

func TestParsePRInput_Number(t *testing.T) {
	owner, repo, number, err := parsePRInput("123")
	if err != nil {
		t.Fatalf("parsePRInput returned error: %v", err)
	}
	if owner != "" || repo != "" {
		t.Fatalf("expected empty owner/repo for numeric input, got %q/%q", owner, repo)
	}
	if number != 123 {
		t.Fatalf("expected number=123, got %d", number)
	}
}

func TestParsePRInput_URL(t *testing.T) {
	owner, repo, number, err := parsePRInput("https://github.com/acme/widgets/pull/42")
	if err != nil {
		t.Fatalf("parsePRInput returned error: %v", err)
	}
	if owner != "acme" || repo != "widgets" || number != 42 {
		t.Fatalf("unexpected parse result: owner=%q repo=%q number=%d", owner, repo, number)
	}
}

func TestSplitOwnerRepo(t *testing.T) {
	owner, repo, err := splitOwnerRepo("acme/widgets.git")
	if err != nil {
		t.Fatalf("splitOwnerRepo returned error: %v", err)
	}
	if owner != "acme" || repo != "widgets" {
		t.Fatalf("unexpected split result: owner=%q repo=%q", owner, repo)
	}
}

func TestMapGitHubStatus(t *testing.T) {
	tests := map[string]string{
		"added":    "A.",
		"removed":  "D.",
		"modified": "M.",
		"renamed":  "R.",
		"copied":   "C.",
		"other":    "..",
	}

	for in, want := range tests {
		if got := mapGitHubStatus(in); got != want {
			t.Fatalf("mapGitHubStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildUnifiedPatch_WrapsPatchBody(t *testing.T) {
	patch := buildUnifiedPatch(prFile{
		Filename: "main.go",
		Status:   "modified",
		Patch:    "@@ -1 +1 @@\n-old\n+new",
	})

	if patch == "" {
		t.Fatal("expected non-empty patch")
	}
	if want := "diff --git a/main.go b/main.go"; !strings.HasPrefix(patch, want) {
		t.Fatalf("expected patch to start with %q, got %q", want, patch)
	}
}

func TestParsePaginatedFilesJSON_SlurpFormat(t *testing.T) {
	body := []byte(`[[{"filename":"a.go","status":"modified","patch":"@@ -1 +1 @@\n-a\n+b"}],[{"filename":"b.go","status":"added","patch":"@@ -0,0 +1 @@\n+b"}]]`)

	files, err := parsePaginatedFilesJSON(body)
	if err != nil {
		t.Fatalf("parsePaginatedFilesJSON returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Filename != "a.go" || files[1].Filename != "b.go" {
		t.Fatalf("unexpected filenames: %#v", files)
	}
}

func TestBuildSubmitReviewPayload_MapsCommentSides(t *testing.T) {
	payload := buildSubmitReviewPayload(Context{HeadSHA: "abc123"}, "summary", "APPROVE", []comments.Comment{
		{Path: "a.go", Side: comments.SideNew, Line: 7, Body: "new-side"},
		{Path: "a.go", Side: comments.SideOld, Line: 3, Body: "old-side"},
	})

	if payload.Event != "APPROVE" {
		t.Fatalf("expected APPROVE event, got %q", payload.Event)
	}
	if payload.Body != "summary" {
		t.Fatalf("expected custom review body")
	}
	if payload.CommitID != "abc123" {
		t.Fatalf("expected commit id to match head sha")
	}
	if len(payload.Comments) != 2 {
		t.Fatalf("expected 2 payload comments, got %d", len(payload.Comments))
	}
	if payload.Comments[0].Side != "RIGHT" {
		t.Fatalf("expected new-side comment mapped to RIGHT, got %q", payload.Comments[0].Side)
	}
	if payload.Comments[1].Side != "LEFT" {
		t.Fatalf("expected old-side comment mapped to LEFT, got %q", payload.Comments[1].Side)
	}
}
