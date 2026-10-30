# CLAUDE.md

This file provides guidance to agents when working with code in this repository.

## Overview

`jt` is a Go terminal kanban board for Jira, built with bubbletea/lipgloss/bubbles/glamour. It imports `github.com/ankitpokhrel/jira-cli`'s `pkg/jira` client as a library, but builds that client from its own settings and never reads jira-cli's config.

## Commands

The Nix flake provides the dev shell (direnv is set up via `.envrc`) and sets `GOFLAGS=-timeout=10s`.

- Build/run: `go build ./...` / `go run .`
- Test all: `go test ./...`
- Single test: `go test ./internal/tui -run TestName`
- Lint: `golangci-lint run` (config in `.golangci.yml`; formatters are gofumpt and goimports)
- Nix build: `nix build`. After changing `go.mod`/`go.sum`, update `vendorHash` in `flake.nix`.

## Architecture

Two layers, wired together by the cobra commands in `cmd/`, plus `jt init`. `main.go` only calls `cmd.Execute`. `cmd/root.go` runs the root command through fang, which styles help and errors with the current tint, so the settings file and theme load before any command runs. The root command opens the board, and `cmd/init.go` runs `internal/setup`. Each command gets its own file in `cmd/`.

- `internal/config`: jt's settings file (`$JT_CONFIG_FILE`, else `jt.yaml` in the user config dir), read through its own viper instance. The `jira` section holds the connection settings, default project and board. The API token lives only in the OS keyring under the `jt` service, keyed by login.
- `internal/jira`: wraps jira-cli's `pkg/jira` client. `New` takes the `config.Jira` settings and the token. Don't use jira-cli's `api` package, since it reads jira-cli's config through the global viper. Only Jira Cloud is supported. The default project is `jira.project`, and `jira.board_id` is used for it when set. `board.go` picks the board otherwise, builds the JQL, and maps board column config to `Column`s. Scrum boards load open sprints. Kanban boards load open work plus issues resolved in the last 14 days.
- `internal/setup`: `jt init`. A sequence of standalone huh forms and spinners that collects the site, email and token, checks them with `CheckLogin`, picks a project and board, then saves to the keyring and settings file.
- `internal/tui`: the bubbletea UI. The root `Model` in `app.go` owns the board state (`boardNav` in `nav.go`), the project `picker`, and at most one `popup` (issue view, comments, move menu). It depends on the `Backend` interface defined in `app.go`, not on `*jira.Client`, so tests use fakes. Network calls run as `tea.Cmd`s that return `*LoadedMsg`/`*DoneMsg` types. `Update` routes them, and popups receive their results through `forwardToPopup`.

`system.go` holds the side-effecting helpers (clipboard with an OSC 52 and tmux fallback, open in browser). They are injected into `Model` as `copyText` and `openURL` so tests can stub them.

Quit behavior: `ctrl-d` exits immediately. `ctrl-c` sets `confirmQuit`, and a second `ctrl-c` exits.

## Conventions

- Tests sit next to the code and focus on model and state transitions. Avoid excessive comments and repetitive tests.
- Imports are grouped as stdlib, third-party, then `github.com/hawkbawk/jt/...`.
