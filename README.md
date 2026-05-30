# claude-code-statusline

A powerline status line for Claude Code. Reads the status JSON Claude Code
sends on stdin and prints a rendered line.

Shows: model, git branch, context window usage, cost, rate limit, session id,
and elapsed time. Collapses to two lines when the terminal is too narrow.

## Install

```
go install github.com/birdayz/claude-code-statusline@latest
claude-code-statusline --install
```

The first command drops the binary in `$(go env GOPATH)/bin` (put that on your
PATH). The second writes itself into `~/.claude/settings.json`.

## Requirements

`git` for the branch segment. `gh`, authenticated, for the PR segment. Missing
either just drops that segment. Nothing is configured, nothing is stored.
