# jt

`jt` is a terminal kanban board for Jira. It shows the issues for a project as columns on a board, and lets you view, comment on, move, and reassign them without leaving the terminal.

`jt` keeps its Jira connection in its own settings file and its API token in your OS keyring.

## Built with

`jt` is a thin layer on top of other people's work, and it wouldn't exist without them. Thank you to:

- [jira-cli](https://github.com/ankitpokhrel/jira-cli) by Ankit Pokhrel, which `jt` imports as a library for all Jira access.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), and [Glamour](https://github.com/charmbracelet/glamour) from Charm, which make up the terminal UI.
- [bubbletint](https://github.com/lrstanley/bubbletint), which handles almost all of the theming code.

## Install

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

To pin a release, add its tag to the flake reference:

```sh
nix run github:Hawkbawk/jt/v0.1.0
```

In a flake, use `inputs.jt.url = "github:Hawkbawk/jt/v0.1.0";`.

The flake is configured with a Cachix cache, so prebuilt binaries are available for x86_64-linux, aarch64-linux, and aarch64-darwin.

### From a release tarball

Each [release](https://github.com/Hawkbawk/jt/releases) has tarballs for linux-amd64, linux-arm64, and darwin-arm64, plus a `checksums.txt`. Download the one for your platform, verify it, and put `jt` on your `PATH`:

```sh
version=0.1.0
platform=darwin_arm64 # or linux_amd64, linux_arm64
curl -LO "https://github.com/Hawkbawk/jt/releases/download/v${version}/jt_v${version}_${platform}.tar.gz"
curl -LO "https://github.com/Hawkbawk/jt/releases/download/v${version}/checksums.txt"
sha256sum --check --ignore-missing checksums.txt # on macOS: shasum -a 256 --check --ignore-missing checksums.txt
tar -xzf "jt_v${version}_${platform}.tar.gz"
install jt ~/.local/bin/
```

### From source

```sh
git clone git@github.com:Hawkbawk/jt.git
cd jt
go build -o jt .
```

## Run

Once the [Jira connection](#jira) is set up, start `jt` from any directory:

```sh
jt
```

`jt` opens the default project from your settings (`jira.project`). If `jira.board_id` is set, `jt` uses that board for the default project. Otherwise it picks a board for the project. Scrum boards show open sprints. Kanban boards show open work plus issues resolved in the last 14 days.

Run `jt --version` to see the installed version and `jt --help` for usage.

## Settings

`jt` keeps its own settings in `jt.yaml` in your user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS). It creates the file the first time you run `jt`. Set `JT_CONFIG_FILE` to use a different path.

`jt` reads the file once at startup. If you edit it by hand, restart `jt` to pick up the change.

### Jira

The `jira` section says how to reach your Jira Cloud site:

```yaml
jira:
  server: https://example.atlassian.net
  login: you@example.com # the email you sign in to Jira with
  project: ABC
  board_id: 42 # optional
```

`jt` signs in with your login email and an API token.

The API token isn't stored in the file. `jt` reads it from your OS keyring, under the service `jt` and your login as the account.

### Theme

Colors come from [bubbletint](https://github.com/lrstanley/bubbletint)'s 770+ tints ([browse them](https://lrstanley.github.io/bubbletint/)). Press `t` in `jt` to pick one. The list is paged and filterable (press `/`), and picking a tint applies it right away and saves its ID to the settings file:

```yaml
theme: gruvbox_dark_hard
```

Leave `theme` empty to follow your terminal's theme, or press `r` in the picker to go back to it. `jt` reads the terminal's config and updates live when you change it. This works in kitty, Ghostty, Alacritty, foot, Windows Terminal and terminals configured through X resources. In other terminals, `jt` starts on Dracula+. An unknown `theme` falls back the same way, with a warning in the status line.

## Development

The Nix dev shell (loaded automatically if you use direnv) provides Go and the lint tools.

```sh
go build ./...
go test ./...
golangci-lint run
```
