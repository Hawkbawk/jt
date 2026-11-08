# jt

`jt` is a terminal UI for Jira. It shows a project's issues as a kanban board, and has a search screen for finding issues across projects. From either one you can view, comment on, move, assign and edit issues without leaving the terminal.

`jt` works with Jira Cloud. Run `jt init` once to connect it, and it keeps the connection in its own settings file and your API token in your OS keyring.

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

Nix only applies a flake's cache settings for trusted users. If you see a warning about an untrusted substituter, Nix is building from source instead. To use the cache, either add your user to `trusted-users` in your Nix config and accept the flake's settings when prompted (or pass `--accept-flake-config`), or run `cachix use hawkbawk-jt` as root to add the cache system-wide.

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

First, connect `jt` to your Jira Cloud site:

```sh
jt init
```

It asks for your site URL, your email and an [API token](https://id.atlassian.com/manage-profile/security/api-tokens), checks that they work, and then lets you pick a default project and board. Run it again any time to change them.

Then start `jt` from any directory:

```sh
jt
```

`jt` opens the default project from your settings (`jira.project`). If `jira.board_id` is set, `jt` uses that board for the default project. Otherwise it picks a board for the project. Scrum boards show open sprints. Kanban boards show open work plus issues resolved in the last 14 days.

Press `f` on the board to choose a quick filter or assignee. Use `tab` / `shift+tab` to switch lists, `/` to search the current list, and `enter` to apply the highlighted option. Select "All issues" to clear the filter. Press `c` on a selected issue to run a configured command; `C` opens its comments.

### Search

`jt search` (or `jt s`) opens a search screen. Fill in any of the filters and press `ctrl+s` to see matching issues below the form (up to 100). You can filter by issue key, project, issue type, sprint, and created or updated date. Dates can be a relative time like `-2d`, an absolute date like `2026-03-01`, or a range like `2026-01-01..2026-02-01`. Results can be sorted by a field in either direction.

If the filters aren't enough, press `ctrl+t` to write JQL directly. Switching with an empty box starts you from the JQL the form would have built.

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | Next / previous field |
| `ctrl+s` | Run the search |
| `ctrl+t` | Switch between basic filters and JQL |
| `ctrl+o` | Move between the form and the results |

In the results, the same keys as the board work on the selected issue: `enter` or `v` to view it, `C` for comments, `m` to move it, `a` to assign it, `e` to edit it, `y` to copy its URL, `o` to open it in the browser and `r` to run the search again. `esc` goes back to the form.

Run `jt --version` to see the installed version and `jt --help` for usage.

## Settings

`jt` keeps its own settings in `jt.yaml` in your user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS). It creates the file the first time you run `jt`. Set `JT_CONFIG_FILE` to use a different path.

`jt` reads the file once at startup. If you edit it by hand, restart `jt` to pick up the change.

### Jira

`jt init` writes the `jira` section, which says how to reach your Jira Cloud site. You can also edit it by hand:

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

### Commands

Add commands to `jt.yaml` to run them on the selected issue with `c`:

```yaml
commands:
  - name: Open issue in VS Code
    executable: code
    args:
      - "{{.Key}}"
      - "{{.URL}}"
  - executable: gh
    args:
      - issue
      - view
      - "{{.Key}}"
```

Each argument is rendered separately with Go `text/template` syntax; the executable is run directly, without a shell. The optional `name` is shown in the picker and defaults to the executable. Use `executable` for its path (the synonymous `command` key is also accepted). Templates can use `.Key` (or `.IssueKey`), `.Project`, `.Board`, `.URL`, `.Title`, `.Description`, and `.Comments`. Comments are Jira comments with `.Author`, `.Created`, `.Edited`, and `.Body` fields, so you can use `{{range .Comments}}{{.Body}}{{end}}`.

## Development

The Nix dev shell (loaded automatically if you use direnv) provides Go and the lint tools.

```sh
go build ./...
go test ./...
golangci-lint run
```
