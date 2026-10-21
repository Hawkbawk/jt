package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
)

// copier copies text with the native clipboard, falling back to an OSC 52
// escape sequence that asks the terminal to copy.
type copier struct {
	getenv func(string) string
	native func(string) error
	// tmux returns the set-clipboard and allow-passthrough options.
	tmux func() (setClipboard, passthrough string, err error)
	term io.Writer
}

// copyToClipboard uses pbcopy on macOS and wl-copy, xclip or xsel on Linux,
// falling back to OSC 52.
func copyToClipboard(s string) error {
	return copier{
		getenv: os.Getenv,
		native: clipboard.WriteAll,
		tmux:   tmuxOptions,
		term:   os.Stdout,
	}.copy(s)
}

// copy skips the native clipboard over SSH, where it would copy on the
// remote machine instead of the user's.
func (c copier) copy(s string) error {
	if c.getenv("SSH_TTY") == "" && c.getenv("SSH_CONNECTION") == "" && c.native(s) == nil {
		return nil
	}
	seq := osc52.New(s)
	if c.getenv("TMUX") != "" {
		clip, pass, err := c.tmux()
		switch {
		case err != nil:
			return fmt.Errorf("reading tmux options: %w", err)
		case clip == "on":
			// tmux forwards the sequence to the outer terminal itself.
		case pass == "on" || pass == "all":
			seq = seq.Tmux()
		default:
			return fmt.Errorf("tmux set-clipboard is %q; set it to on", clip)
		}
	}
	_, err := seq.WriteTo(c.term)
	return err
}

// tmuxOptions reads the options as they apply to the current pane.
func tmuxOptions() (string, string, error) {
	args := []string{"display-message", "-p"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	out, err := exec.Command("tmux", append(args, "#{set-clipboard} #{allow-passthrough}")...).Output()
	if err != nil {
		return "", "", err
	}
	clip, pass, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	return clip, pass, nil
}

// openInBrowser opens url with the platform's default handler.
func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Run()
}
