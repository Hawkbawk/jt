package fakejira

import "github.com/hawkbawk/jt/internal/config"

// DemoCommands returns commands for trying issue actions in fake mode.
func DemoCommands() []config.Command {
	return []config.Command{
		{
			Name:    "Open ticket details in Neovim (new tmux window)",
			Command: "tmux",
			Args: []string{
				"new-window",
				"-n",
				"{{.Key}}",
				"-e",
				"JT_DEMO_KEY={{.Key}}",
				"-e",
				"JT_DEMO_TITLE={{.Title}}",
				"-e",
				"JT_DEMO_URL={{.URL}}",
				`nvim -c 'setlocal buftype=nofile bufhidden=wipe' -c 'call setline(1, ["Fake Jira ticket", "Key: " . $JT_DEMO_KEY, "Title: " . $JT_DEMO_TITLE, "URL: " . $JT_DEMO_URL])'`,
			},
		},
		{
			Name:    "Show ticket summary in tmux",
			Command: "tmux",
			Args:    []string{"display-message", "Fake Jira: {{.Key}} — {{.Title}}"},
		},
		{
			Name:    "Copy ticket URL to tmux buffer",
			Command: "tmux",
			Args:    []string{"set-buffer", "{{.URL}}"},
		},
	}
}
