# jt

`jt` is a terminal kanban board for Jira. It shows the issues for a project as columns on a board, and lets you view, comment on, move, and reassign them without leaving the terminal.

`jt` has no config or login of its own. It reuses the configuration and credentials from [jira-cli](https://github.com/ankitpokhrel/jira-cli), so you need to set that up first.

## Built with

`jt` is a thin layer on top of other people's work, and it wouldn't exist without them. Thank you to:

- [jira-cli](https://github.com/ankitpokhrel/jira-cli) by Ankit Pokhrel, which `jt` imports as a library for all Jira access, config, and auth.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), and [Glamour](https://github.com/charmbracelet/glamour) from Charm, which make up the terminal UI.

## Install

You need a working jira-cli setup. Follow the [jira-cli installation guide](https://github.com/ankitpokhrel/jira-cli#installation), then run `jira init` and follow the prompts.

### With Go

```sh
go install github.com/hawkbawk/jt@latest
```

### With Nix

```sh
nix run github:Hawkbawk/jt
```

To install it into your profile instead:

```sh
nix profile install github:Hawkbawk/jt
```

The flake is configured with a Cachix cache, so prebuilt binaries are available for x86_64-linux, aarch64-linux, and aarch64-darwin.

### From source

```sh
git clone git@github.com:Hawkbawk/jt.git
cd jt
go build -o jt .
```

## Run

After `jira init` has been run, start `jt` from any directory:

```sh
jt
```

`jt` opens the default project from your jira-cli config (`project.key`). If the configured `board.id` belongs to that project, `jt` uses it. Otherwise it picks a board for the project. Scrum boards show open sprints. Kanban boards show open work plus issues resolved in the last 14 days.

If your jira-cli config lives somewhere other than the default location, set `JIRA_CONFIG_FILE` to its path.

Press `ctrl-d` to quit immediately, or `ctrl-c` twice to confirm quitting.

## Settings

`jt` keeps its own settings in `jt.yaml` in your user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS). It creates the file the first time you run `jt`. Set `JT_CONFIG_FILE` to use a different path.

`jt` reads the file once at startup. If you edit it by hand, restart `jt` to pick up the change.

### Theme

Colors come from [bubbletint](https://github.com/lrstanley/bubbletint)'s 770+ tints ([browse them](https://lrstanley.github.io/bubbletint/)). Press `t` in `jt` to pick one. The list is paged and filterable (press `/`), and picking a tint applies it right away and saves its ID to the settings file:

```yaml
theme: gruvbox_dark_hard
```

Leave `theme` empty to follow your terminal's theme, or press `r` in the picker to go back to it. `jt` reads the terminal's config and updates live when you change it. This works in kitty, Ghostty, Alacritty, foot, Windows Terminal and terminals configured through X resources. In other terminals, `jt` starts on Dracula+. A picked tint holds until you change your terminal's theme, which `jt` then follows. An unknown `theme` falls back the same way, with a warning in the status line.

## Development

The Nix dev shell (loaded automatically if you use direnv) provides Go and the lint tools.

```sh
go build ./...
go test ./...
golangci-lint run
```
