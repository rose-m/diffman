# LeDiff

TUI-based diff viewing and annotation tool.

## Leader Commands

LeDiff supports custom command launchers via a Vim-style leader key.

- Press `<space>` then a configured key to run a command.
- Config file path: `~/.config/lediff/config.json`
  - If `XDG_CONFIG_HOME` is set, LeDiff uses `$XDG_CONFIG_HOME/lediff/config.json`.
- Leader commands receive:
  - `$ROOT`: repository root path
  - `$FILE`: currently selected diff file path (repo-relative)

Example:

```json
{
  "leader_commands": {
    "g": "lazygit",
    "o": "code \"$ROOT/$FILE\""
  }
}
```
