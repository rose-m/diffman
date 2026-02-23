# diffman

`diffman` is a terminal UI for reviewing git diffs and attaching per-line review comments.

It is built with Bubble Tea and designed for fast keyboard-driven review.

https://github.com/user-attachments/assets/3cd5cae6-3137-4b85-bad8-30e8eddcc5bd

## Features

- File tree for all changed files in the current repository.
- Side-by-side diff view with syntax-like coloring and word-level change highlighting.
- Per-line comments stored locally per repository.
- Inline comment display in the diff panes.
- Comment list view across all files.
- Export comments to clipboard in a review-friendly format.
- Leader key commands (`<space><key>`) configurable via user config.

## Requirements

- Go `1.26+`
- `git`
- A terminal with Unicode/color support

## Installation

Build and install into `~/.local/bin`:

```bash
make install
```

Other targets:

```bash
make run
make build
make test
make lint
```

## Quick Start

Run `diffman` anywhere inside a git repository:

```bash
diffman
```

`diffman` discovers the repository root automatically and shows changed files.

## UI Overview

The app has three views:

- Files view: directory tree + changed files.
- Diff view: old/new diff panes (or single pane for one-sided diffs).
- Comments view: all comments across files.

Focus moves with `tab`.

## Keybindings

### Global

- `tab`: switch focus (files -> diff -> comments)
- `m`: toggle comments view
- `r`: refresh files/diff state
- `t`: toggle diff mode (`all`, `unstaged`, `staged`)
- `C`: clear all comments (with confirmation)
- `<space><key>`: run configured leader command
- `?`: toggle expanded help
- `q`: quit (except in comments view, where it closes comments view)

### Files View

- `j` / `k`: move selection (diff updates immediately)
- `ctrl+e` / `ctrl+y`: scroll file tree window
- `h`: parent/collapse behavior
- `l`: child/expand behavior; on file, focus diff view
- `enter`: open file diff; on directory, toggle collapse
- `z`: toggle file pane width (`40` <-> `120`)

Directory navigation behavior:

- On a file, `h` goes to its parent directory.
- On an expanded directory, `h` collapses it.
- On a collapsed directory, `h` goes to parent.
- On a directory, `l` expands and jumps to first direct child.

### Diff View

- `j` / `k`: move diff cursor
- `ctrl+e` / `ctrl+y`: scroll window by one line
- `ctrl+f` / `ctrl+b`: page down/up
- `g` / `G`: top/bottom
- `c`: add comment on current line
- `e`: edit comment on current line
- `d`: delete comment on current line
- `n` / `p`: jump next/previous comment in current diff
- `y`: copy exported comments to clipboard
- `z` or `l`: hide/show file pane
- `h`: focus files view

### Comments View

- `j` / `k`: move
- `ctrl+e` / `ctrl+y`: scroll window by one line
- `ctrl+f` / `ctrl+b`: page down/up
- `g` / `G`: top/bottom
- `enter`: jump to selected comment in diff (if not stale)
- `e`: edit selected comment
- `d`: delete selected comment
- `m` or `q`: close comments view

## Comments and Persistence

Comments are saved in the repo git directory:

- `.git/.diffman/comments.json`

Each comment is anchored by:

- file path
- side (`old` or `new`)
- line number

`diffman` shows inline comment text beneath the commented line in diff panes.

## Stale Comments

A comment is marked stale when its anchor can no longer be found in current diff output.

Behavior:

- Stale comments are marked in comments view.
- Jumping to stale comments is disabled.
- Stale comments are excluded from clipboard export.
- A warning appears in the footer when stale comments exist.

## Diff Modes

Toggle with `t`:

- `all`: default, includes all changes
- `unstaged`: unstaged only
- `staged`: staged only

For one-sided changes:

- New file: only the `New` pane is shown.
- Deleted file: only the `Old` pane is shown.

## Leader Commands (Config)

Configure custom command shortcuts in:

- `$HOME/.config/diffman/config.json`
- or `$XDG_CONFIG_HOME/diffman/config.json` if `XDG_CONFIG_HOME` is set

Format:

```json
{
  "leader_commands": {
    "g": "lazygit",
    "o": "code \"$ROOT/$FILE\""
  }
}
```

Rules:

- Keys must be exactly one character.
- Values are shell commands.
- Commands run via `$SHELL -lc '<command>'`.

Environment variables provided to leader commands:

- `ROOT`: absolute repository root
- `FILE`: currently selected file path (repo-relative)

After a leader command exits, `diffman` auto-refreshes file/diff state.

## Clipboard Export Format

`y` copies non-stale comments in this style:

```text
Review comments:

1) path/to/file.go new:21: Comment text
```

With context block per comment when available.

## Notes

- `diffman` only shows files reported as changed by `git status`.
- Running outside a git repository will fail at startup.
- If config parsing fails, the app still starts and shows an alert.
