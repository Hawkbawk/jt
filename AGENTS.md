# CLAUDE.md

This file provides guidance to agents when working with code in this repository.

## Overview

`jt` is a Go terminal kanban board for Jira, built with bubbletea/lipgloss/bubbles/glamour. It imports `github.com/ankitpokhrel/jira-cli`'s `pkg/jira` client as a library, but builds that client from its own settings and never reads jira-cli's config.

## Commands

The Nix flake provides the dev shell (direnv is set up via `.envrc`) and sets `GOFLAGS=-timeout=10s`.

- Build/run: `go build ./...` / `go run .`
- Test all: `go test ./...`
- Single test: `go test ./internal/tui/board -run TestName`
- Lint: `golangci-lint run` (config in `.golangci.yml`; formatters are gofumpt and goimports)
- Demo data: `go run . --fake` runs on an in-memory backend without the settings file or keyring. It prints its seed; `--fake-seed N` repeats a run, and `--fake-users` and `--fake-issues` set the size.
- Nix build: `nix build`. After changing `go.mod`/`go.sum`, update `vendorHash` in `flake.nix`.

## Architecture

Two layers, wired together by the cobra commands in `cmd/`, plus `jt init`. `main.go` only calls `cmd.Execute`. `cmd/root.go` runs the root command through fang, which styles help and errors with the current tint, so the settings file and theme load before any command runs. The root command opens the board, and `cmd/init.go` runs `internal/setup`. Each command gets its own file in `cmd/`.

- `internal/config`: jt's settings file (`$JT_CONFIG_FILE`, else `jt.yaml` in the user config dir), read through its own viper instance. The `jira` section holds the connection settings, default project and board. The API token lives only in the OS keyring under the `jt` service, keyed by login.
- `internal/jira`: wraps jira-cli's `pkg/jira` client. `New` takes the `config.Jira` settings and the token. Don't use jira-cli's `api` package, since it reads jira-cli's config through the global viper. Only Jira Cloud is supported. The default project is `jira.project`, and `jira.board_id` is used for it when set. `board.go` picks the board otherwise, builds the JQL, and maps board column config to `Column`s. Scrum boards load open sprints. Kanban boards load open work plus issues resolved in the last 14 days.
- `internal/setup`: `jt init`. A sequence of standalone huh forms and spinners that collects the site, email and token, checks them with `CheckLogin`, picks a project and board, then saves to the keyring and settings file.
- `internal/tui/board`: the board screen. `board.Model` in `app.go` owns the board state (`boardNav` in `nav.go`) and the popup host. `internal/tui/search` holds `search.Model`, the separate search screen. Both embed `screen.Layout` (window, spinner, status, help and the popup host) and reuse `screen.IssueActions`. They depend on their own `Backend` interfaces, not on `*jira.Client`, so tests use fakes from `internal/tui/tuitest`. Network calls run as `tea.Cmd`s that return `*LoadedMsg`/`*DoneMsg` types, and `Update` routes them.

`internal/tui/popup` hosts any `tea.Model` as a modal (project picker, board picker, issue view, comments, move, assign, edit, filter, theme). There are no open or close methods: `popup.Open(model)` and `popup.Close()` return commands, and the host handles the resulting messages in its `Update`. Components under `internal/components` never import `popup`. They end by returning a `tuimsg.Final` message (`tuimsg.End(result)`, or `tuimsg.Cancel()` for `tuimsg.Cancelled`), and the host turns it into closing the popup and passes the result, but not `Cancelled`, on to the screen. A component starts its own loads in `Init`, embeds `tuimsg.Requests` and passes each request through `Request`. When the popup opens, the host calls `WrapRequests` so it can tag requests with the popup's generation and drop responses for a popup that has closed or been replaced, so content never checks whose response it got. Without a host, `Request` returns the command unchanged, which is what component tests rely on. A component that holds another one (issuecomments and issuedetail hold an `issueview.Model`) overrides `WrapRequests` to pass the wrapper on. The host sizes its content with a `tea.WindowSizeMsg` for the area inside the frame, which it draws, and passes everything else through to the content, so screens give the host every message they don't consume (`UpdatePopup`) and let `popup.Captures` keep input from reaching the screen behind it. Screens handle the messages they share (spinner ticks, popup open and close, theme saved, `ActionDoneMsg`) with `screen.Layout.HandleCommon`. Any other message a component returns reaches the screen first; the projectpicker sends its `LoadedMsg` that way so the board can keep the projects. A component reports a change to an issue by ending with a message that implements `tuimsg.Finished` (`issueassign.Assigned`, `issuemove.Moved`, `issueedit.Edited`), which the screens turn into a reload. Drawing helpers that don't depend on being in a popup live in `internal/tui/layout` (`Header`, `StatusLine`, `ChromeLines`, `Stack`, which pins a footer under a header and body, and the `KeyMap` with its `Dismiss` binding). `internal/tui/widget` holds the shared spinner and forms; spinners are `widget.Spinner`s: pass them ticks with `Update(msg, busy)` and call `Sync(busy)` after anything that can make the owner busy. `internal/components/issueview` is a `tea.Model` that loads either an issue (details) or its comments, only when it opens, and shows them as scrollable text under a title. Its owner sizes it for the room left above its own footer, draws the footer, and reloads it after a change.

`internal/fakejira` is an in-memory, stateful implementation of `board.Backend` and `search.Backend` filled with demo data that gofakeit generates from a seed (`WithSeed`, else random) and a `Size` (`WithSize`). Seed structs in `generate.go` carry `fake` tags, with custom lookups (`jirauser`, `jirastatus`, `chance` and others) for the jira types; references between generated data are picked afterwards. The root command's `--fake` flag uses it, and only works in development builds, which have no `version` set by ldflags. `Execute` checks for the flag before loading settings, and theme changes go to `sessionSettings`, which saves nothing. When a screen's `Backend` gains a method, implement it in the fake too.

`internal/system` holds the side-effecting helpers (clipboard with an OSC 52 and tmux fallback, open in browser). They are injected into the shared `screen.IssueActions` as `CopyText` and `OpenURL` so tests can stub them.

Search uses one embedded huh form for basic filters or advanced JQL. The basic form is two columns (`huh.LayoutColumns`) of single selects and validated inputs; huh runs each field's validator as you leave it. ctrl+s sends huh an enter first so the focused field validates and shows its own error, and only searches if it has none. huh owns all focus movement; when it finishes the form (enter on the last field) jt searches and rebuilds the form to start over. Results (even none) show in a bubble-table (`search/searchresults.go`) below the form, with "No issues found." or the error beneath; each new search resets the cursor. `inResults` is the only focus state: ctrl+o toggles it, esc/tab in the results returns to the form, and focusing results blurs the form's current field. `Backend.Search` takes no context; new searches and refreshes are ignored while one is running.

The board picker (`ctrl+b`) switches the open project's board for the session, and `boardPane` remembers the choice per project. `f` in the board picker favorites the highlighted board as `jira.board_id` through `Settings.SaveBoard`, and only for the default project. `f` on the board opens the quick filter and assignee popup. `f` in the project picker saves `jira.project` through `Settings.SaveProject`, which clears `jira.board_id` when the project changes. Both also update the backend (`SetDefaultProject`, `SetDefaultBoard`) so the session sees the new default.

Quit behavior: `ctrl-d` exits immediately. `ctrl-c` sets `confirmQuit`, and a second `ctrl-c` exits.

## Conventions

- Tests sit next to the code and focus on model and state transitions. Avoid excessive comments and repetitive tests.
- Imports are grouped as stdlib, third-party, then `github.com/hawkbawk/jt/...`.
