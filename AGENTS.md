# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`jt` is a Go terminal kanban board for Jira, built with bubbletea/lipgloss/bubbles/glamour. It does not talk to Jira directly. It imports `github.com/ankitpokhrel/jira-cli` as a library and reuses its config and auth, so `jira init` must have been run first.
## Commands

The Nix flake provides the dev shell (direnv is set up via `.envrc`) and sets `GOFLAGS=-timeout=10s`.

- Build/run: `go build ./...` / `go run .`
- Test all: `go test ./...`
- Single test: `go test ./internal/tui -run TestName`
- Lint: `golangci-lint run` (config in `.golangci.yml`; formatters are gofumpt and goimports)
- Nix build: `nix build`. After changing `go.mod`/`go.sum`, update `vendorHash` in `flake.nix`.

## Architecture

Two layers, wired together in `main.go`:

- `internal/jira`: wraps jira-cli's client. `New` loads jira-cli's own config (`$JIRA_CONFIG_FILE`, else `$XDG_CONFIG_HOME/.jira/.config.yml`) through the global viper instance. jt has no config of its own: the default project is jira-cli's `project.key`, and its `board.id` is used when it belongs to the project. `board.go` picks the board otherwise, builds the JQL, and maps board column config to `Column`s. Scrum boards load open sprints. Kanban boards load open work plus issues resolved in the last 14 days.
- `internal/tui`: the bubbletea UI. The root `Model` in `app.go` owns the board state (`boardNav` in `nav.go`), the project `picker`, and at most one `popup` (issue view, comments, move menu). It depends on the `Backend` interface defined in `app.go`, not on `*jira.Client`, so tests use fakes. Network calls run as `tea.Cmd`s that return `*LoadedMsg`/`*DoneMsg` types. `Update` routes them, and popups receive their results through `forwardToPopup`.

`system.go` holds the side-effecting helpers (clipboard with an OSC 52 and tmux fallback, open in browser). They are injected into `Model` as `copyText` and `openURL` so tests can stub them.

Quit behavior: `ctrl-d` exits immediately. `ctrl-c` sets `confirmQuit`, and a second `ctrl-c` exits.

## Conventions

- Tests sit next to the code and focus on model and state transitions. Avoid excessive comments and repetitive tests.
- Imports are grouped as stdlib, third-party, then `github.com/hawkbawk/jt/...`.
