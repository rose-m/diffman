# GitHub PR Review Plan for diffman

## Why this plan

`diffman` already has strong local-review primitives (file list, parsed diff rows, inline comments, stale detection, export). The fastest path to “actually review GitHub PRs” is to **reuse those primitives** and swap the data source from local git working tree to GitHub PR data.

---

## Current architecture (as-is)

### Core flow today

1. `app.NewModel()` discovers repo and initializes services (`statusSvc`, `diffSvc`) and local comment store.
2. `loadFilesCmd()` loads changed files from `git status` via `internal/git/status.go`.
3. Selecting a file triggers `loadDiffCmd(path)` which calls `diffSvc.Diff(...)` from `internal/git/diff.go`.
4. Raw unified diff is parsed by `diffview.ParseUnifiedDiff` into `[]DiffRow`.
5. `diffview.RenderSplitWithLayoutComments` renders side-by-side panes with inline comment blocks.
6. Comments are stored locally in `.git/.diffman/comments.json` (`internal/comments/store.go`).
7. `buildCommentStaleMap(...)` re-validates anchors against current parsed diff rows.

### Strong reuse points

- Parser/rendering stack is already PR-ready for unified diffs:
  - `internal/diffview/parse.go`
  - `internal/diffview/render.go`
- App async command/message pattern is already established:
  - `loadFilesCmd`, `loadDiffCmd`, `*_loadedMsg` in `internal/app/model.go`
- Existing comment anchor model (`path + side + line`) aligns with GitHub inline comments.

### Gaps for GitHub PR reviews

- No GitHub client/integration package exists.
- No PR session state in `Model` (repo, PR number, base/head SHAs, review state).
- No submission path from local comments to GitHub review comments.
- No CLI args for PR mode (`cmd/diffman/main.go` is currently zero-arg).

---

## Implementation options

## Option A (recommended): `gh` CLI-backed integration

Use `gh` for auth and API access (no embedded OAuth/token handling in diffman).

**Pros**
- Lowest implementation risk and fastest delivery.
- Auth/session reuse from users’ existing `gh auth login`.
- Fits existing `util.Run(...)` command-execution pattern.

**Cons**
- Runtime dependency on `gh`.
- Need robust parsing/handling for `gh` output and errors.

## Option B: Native GitHub REST client in Go

**Pros**
- Full control, fewer external runtime dependencies.

**Cons**
- Larger scope (auth, token discovery, API plumbing, retries/rate limits).
- Slower to first usable PR-review flow.

**Decision**: Start with **Option A**. Keep interfaces clean so Option B can replace internals later.

---

## Proposed design (minimal, incremental)

## Phase 1 — Read-only PR browsing in diffman

Goal: open a PR and review its diffs in existing UI (no submission yet).

### 1) Introduce PR review source package

Add new package, e.g. `internal/githubpr`:

- `type Service interface { ... }`
- `NewService()` constructor
- Methods (minimum):
  - `ResolvePR(ctx, cwd, input) (PRContext, error)`  // parse number/url and infer repo
  - `ListFiles(ctx, prCtx) ([]git.FileItem, error)`   // adapt to existing file list UI
  - `Diff(ctx, prCtx, path) (string, error)`          // unified diff per file

Keep API style consistent with existing `internal/git` service interfaces.

### 2) Add app PR mode state

In `internal/app/model.go`, add a small PR session struct/state:

- mode: local vs PR
- PR metadata (repo, number, title, base/head, head SHA)

Wire alternate loaders:

- local mode → existing `statusSvc` / `diffSvc`
- PR mode → `githubpr.Service`

Keep downstream parser/render/comment code unchanged.

### 3) Add startup entry for PR mode

In `cmd/diffman/main.go`:

- Add minimal arg parsing:
  - `diffman` (existing local mode)
  - `diffman --pr <number|url>` (PR mode)

Pass options into `app.NewModel(...)` (constructor update required).

### 4) UX updates

- Show PR context in pane title/footer (e.g., `PR #123 owner/repo`).
- Keep current keybindings for navigation/comment drafting.

Deliverable: user can browse GitHub PR diffs with existing diff/comment UI.

---

## Phase 2 — Submit review comments to GitHub

Goal: turn local draft comments into real GitHub PR review comments.

### 1) Add submission API in `internal/githubpr`

Minimum methods:

- `SubmitLineComment(ctx, prCtx, draftComment)`
- `SubmitReview(ctx, prCtx, body, event, draftComments)` (optional in v1, preferred)

Map existing `comments.Comment` (`path`, `side`, `line`, `body`) to GitHub PR review comment payload using PR `head_sha`.

### 2) Add app command/action for submit

Add a keybinding/action in `internal/app/keymap.go` + `Model.Update`:

- Submit all non-stale draft comments to GitHub
- Surface success/failure via existing alert dock

### 3) Draft lifecycle rules

Pick one explicit rule for v1:

- either clear submitted local drafts,
- or mark as submitted (requires model/store extension).

Recommendation for v1: **clear on successful submit** (smallest scope), with confirmation prompt.

Deliverable: user can leave comments in diffman and publish them to PR.

---

## Phase 3 — Review events and thread sync (follow-up)

Goal: complete PR review workflow parity.

- Submit `APPROVE` / `REQUEST_CHANGES` / `COMMENT` review events.
- Load existing PR review threads and show inline/thread markers.
- Resolve/unresolve thread support (if desired).

This phase is intentionally separate to keep first deliverable tight.

---

## File-level change plan

- `cmd/diffman/main.go`
  - add `--pr` option parsing and model initialization wiring.

- `internal/app/model.go`
  - add PR mode/session fields
  - add PR-aware load files/diff commands using existing msg/cmd patterns
  - add submit-review command handling in later phase

- `internal/app/keymap.go`
  - add keybinding(s) for submit/review actions

- `internal/comments/model.go` + `store.go` (optional)
  - only if we choose “mark submitted” rather than “clear after submit”

- `internal/githubpr/*` (new)
  - service interface + `gh` adapter implementation
  - typed models for PR context and submission payloads

---

## Test strategy

1. **Unit tests (new package)**
   - parse/normalize PR input (number/url)
   - map GitHub file list → `[]git.FileItem`
   - diff/submission command construction and error mapping

2. **App tests**
   - PR mode loader routing (file list + diff)
   - submission action success/error alert behavior
   - no regressions in local mode

3. **Existing suites**
   - keep parser/render tests unchanged; they are core safety net

Validation command set:

- `make test`
- `make lint`

---

## Risks and mitigations

- **GitHub API/CLI output variance**
  - Mitigate with strict parsing + fallback errors surfaced in alert dock.

- **Diff/line anchor mismatch when PR updates**
  - Reuse stale-comment detection and require fresh PR reload before submit if needed.

- **Scope creep into full GitHub client**
  - Keep v1 on `gh` adapter behind interface; defer native client.

---

## Suggested milestone sequence

1. PR read-only mode (open PR, list files, view diffs).
2. Submit draft comments as PR comments.
3. Add approve/request-changes event support.
4. Thread sync/resolve UX.

This gives an early, useful end-to-end path while preserving diffman’s current architecture and minimizing churn.
