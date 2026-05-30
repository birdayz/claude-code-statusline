# claude-code-statusline

A powerline status line for Claude Code. Reads the status JSON Claude Code
sends on stdin and prints a single rendered line (or two).

Shows: model, git branch, context window usage, cost, rate limit, session id,
and elapsed time. Two-line mode adds session name and the current PR.

It collapses to two lines when the terminal is too narrow to fit everything.

## Install

```
go install github.com/birdayz/claude-code-statusline@latest
```

That puts the binary in `$(go env GOPATH)/bin`. Make sure that's on your PATH.

Or build it yourself:

```
git clone https://github.com/birdayz/claude-code-statusline
cd claude-code-statusline
go build -o claude-statusline .
```

## Wire it up

Point Claude Code at the binary in `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "claude-code-statusline --two-line"
  }
}
```

Drop `--two-line` if you want everything on a single line.

## Requirements

- `git` on PATH, for the branch segment.
- `gh` on PATH and authenticated, for the PR segment in two-line mode. Without
  it, that segment is simply omitted.

Nothing is configured and nothing is stored. The binary reads stdin, shells out
to `git`/`gh` for the directory Claude Code reports, and prints. That's it.
