package githubpr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	gitint "diffman/internal/git"
	"diffman/internal/util"
)

type ghService struct{}

func (ghService) ResolvePR(ctx context.Context, cwd, input string) (Context, error) {
	owner, repo, number, err := parsePRInput(input)
	if err != nil {
		return Context{}, err
	}
	if owner == "" || repo == "" {
		owner, repo, err = discoverGitHubRepo(ctx, cwd)
		if err != nil {
			return Context{}, err
		}
	}

	var payload struct {
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	body, err := util.Run(ctx, "", "gh", "api", fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number))
	if err != nil {
		return Context{}, err
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return Context{}, fmt.Errorf("parse pr metadata: %w", err)
	}

	return Context{
		Owner:   owner,
		Repo:    repo,
		Number:  number,
		Title:   payload.Title,
		URL:     payload.HTMLURL,
		HeadSHA: payload.Head.SHA,
		HeadRef: payload.Head.Ref,
		BaseRef: payload.Base.Ref,
	}, nil
}

func (ghService) ListFiles(ctx context.Context, pr Context) ([]gitint.FileItem, error) {
	files, err := listPRFiles(ctx, pr)
	if err != nil {
		return nil, err
	}

	out := make([]gitint.FileItem, 0, len(files))
	for _, file := range files {
		out = append(out, gitint.FileItem{
			Path:        file.Filename,
			Status:      mapGitHubStatus(file.Status),
			HasStaged:   false,
			HasUnstaged: true,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})

	return out, nil
}

func (ghService) Diff(ctx context.Context, pr Context, targetPath string) (string, error) {
	files, err := listPRFiles(ctx, pr)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.Filename != targetPath {
			continue
		}
		return buildUnifiedPatch(file), nil
	}

	return "", nil
}

type prFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Patch            string `json:"patch"`
}

func listPRFiles(ctx context.Context, pr Context) ([]prFile, error) {
	body, err := util.Run(
		ctx,
		"",
		"gh",
		"api",
		"--paginate",
		"--slurp",
		fmt.Sprintf("repos/%s/%s/pulls/%d/files?per_page=100", pr.Owner, pr.Repo, pr.Number),
	)
	if err != nil {
		return nil, err
	}

	files, err := parsePaginatedFilesJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	return files, nil
}

func parsePaginatedFilesJSON(body []byte) ([]prFile, error) {
	var files []prFile
	if err := json.Unmarshal(body, &files); err == nil {
		return files, nil
	}

	var pages [][]prFile
	if err := json.Unmarshal(body, &pages); err != nil {
		return nil, fmt.Errorf("parse pr files: %w", err)
	}

	total := 0
	for _, page := range pages {
		total += len(page)
	}
	files = make([]prFile, 0, total)
	for _, page := range pages {
		files = append(files, page...)
	}
	return files, nil
}

func parsePRInput(input string) (owner, repo string, number int, err error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", 0, fmt.Errorf("pull request input cannot be empty")
	}

	if n, convErr := strconv.Atoi(raw); convErr == nil {
		if n <= 0 {
			return "", "", 0, fmt.Errorf("pull request number must be positive")
		}
		return "", "", n, nil
	}

	u, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", "", 0, fmt.Errorf("invalid pull request input %q", input)
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", "", 0, fmt.Errorf("unsupported pull request host %q", u.Host)
	}

	parts := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return "", "", 0, fmt.Errorf("invalid pull request url %q", input)
	}

	n, convErr := strconv.Atoi(parts[3])
	if convErr != nil || n <= 0 {
		return "", "", 0, fmt.Errorf("invalid pull request number in url %q", input)
	}

	return parts[0], parts[1], n, nil
}

func discoverGitHubRepo(ctx context.Context, cwd string) (owner, repo string, err error) {
	out, err := util.Run(ctx, cwd, "git", "config", "--get", "remote.origin.url")
	if err != nil {
		return "", "", err
	}

	raw := strings.TrimSpace(out)
	if raw == "" {
		return "", "", fmt.Errorf("git remote.origin.url is empty")
	}

	if strings.HasPrefix(raw, "git@github.com:") {
		repoPath := strings.TrimPrefix(raw, "git@github.com:")
		return splitOwnerRepo(repoPath)
	}

	if strings.HasPrefix(raw, "https://github.com/") || strings.HasPrefix(raw, "http://github.com/") || strings.HasPrefix(raw, "ssh://git@github.com/") {
		u, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", fmt.Errorf("parse git remote url: %w", parseErr)
		}
		if !strings.EqualFold(u.Host, "github.com") {
			return "", "", fmt.Errorf("unsupported git host %q", u.Host)
		}
		return splitOwnerRepo(strings.TrimPrefix(u.Path, "/"))
	}

	return "", "", fmt.Errorf("unsupported git remote url %q", raw)
}

func splitOwnerRepo(repoPath string) (owner, repo string, err error) {
	clean := strings.TrimSuffix(strings.TrimSpace(repoPath), ".git")
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid github repo path %q", repoPath)
	}
	owner = parts[len(parts)-2]
	repo = parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid github repo path %q", repoPath)
	}
	return owner, repo, nil
}

func mapGitHubStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "added":
		return "A."
	case "removed":
		return "D."
	case "renamed":
		return "R."
	case "copied":
		return "C."
	case "modified", "changed":
		return "M."
	default:
		return ".."
	}
}

func buildUnifiedPatch(file prFile) string {
	patch := strings.TrimSpace(file.Patch)
	if patch == "" {
		return ""
	}
	if strings.HasPrefix(patch, "diff --git ") {
		return patch
	}

	oldPath := file.Filename
	newPath := file.Filename
	switch strings.ToLower(strings.TrimSpace(file.Status)) {
	case "added":
		oldPath = "/dev/null"
		newPath = "b/" + file.Filename
	case "removed":
		oldPath = "a/" + file.Filename
		newPath = "/dev/null"
	case "renamed":
		oldName := strings.TrimSpace(file.PreviousFilename)
		if oldName == "" {
			oldName = file.Filename
		}
		oldPath = "a/" + oldName
		newPath = "b/" + file.Filename
	default:
		oldPath = "a/" + file.Filename
		newPath = "b/" + file.Filename
	}

	header := []string{
		fmt.Sprintf("diff --git a/%s b/%s", file.Filename, file.Filename),
		"--- " + oldPath,
		"+++ " + newPath,
	}
	return strings.Join(header, "\n") + "\n" + patch + "\n"
}
