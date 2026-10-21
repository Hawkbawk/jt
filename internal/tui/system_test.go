package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/aymanbagabas/go-osc52/v2"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func failNative(string) error { return errors.New("no xclip") }

func tmuxWith(clip, pass string) func() (string, string, error) {
	return func() (string, string, error) { return clip, pass, nil }
}

func TestCopyUsesNativeClipboardLocally(t *testing.T) {
	var term bytes.Buffer
	var copied string
	c := copier{getenv: env(nil), native: func(s string) error { copied = s; return nil }, term: &term}
	if err := c.copy("url"); err != nil {
		t.Fatal(err)
	}
	if copied != "url" || term.Len() != 0 {
		t.Fatalf("copied=%q term=%q", copied, term.String())
	}
}

func TestCopyFallsBackToOSC52WhenNativeFails(t *testing.T) {
	var term bytes.Buffer
	c := copier{getenv: env(nil), native: failNative, term: &term}
	if err := c.copy("url"); err != nil {
		t.Fatal(err)
	}
	if term.String() != osc52.New("url").String() {
		t.Fatalf("term=%q", term.String())
	}
}

func TestCopyUsesOSC52OverSSH(t *testing.T) {
	var term bytes.Buffer
	c := copier{
		getenv: env(map[string]string{"SSH_TTY": "/dev/pts/0"}),
		native: func(string) error { t.Fatal("native clipboard used over SSH"); return nil },
		term:   &term,
	}
	if err := c.copy("url"); err != nil {
		t.Fatal(err)
	}
	if term.String() != osc52.New("url").String() {
		t.Fatalf("term=%q", term.String())
	}
}

func TestCopyInTmux(t *testing.T) {
	inTmux := env(map[string]string{"TMUX": "/tmp/tmux-1/default,1,0"})
	tests := []struct {
		name, clip, pass string
		want             string
		wantErr          string
	}{
		{name: "set-clipboard on", clip: "on", pass: "off", want: osc52.New("url").String()},
		{name: "passthrough", clip: "external", pass: "on", want: osc52.New("url").Tmux().String()},
		{name: "passthrough all", clip: "off", pass: "all", want: osc52.New("url").Tmux().String()},
		{name: "unsupported", clip: "external", pass: "off", wantErr: `set-clipboard is "external"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var term bytes.Buffer
			c := copier{getenv: inTmux, native: failNative, tmux: tmuxWith(tt.clip, tt.pass), term: &term}
			err := c.copy("url")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				if term.Len() != 0 {
					t.Fatalf("wrote %q despite error", term.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if term.String() != tt.want {
				t.Fatalf("term=%q want %q", term.String(), tt.want)
			}
		})
	}
}

func TestCopyReportsTmuxQueryFailure(t *testing.T) {
	c := copier{
		getenv: env(map[string]string{"TMUX": "x"}),
		native: failNative,
		tmux:   func() (string, string, error) { return "", "", errors.New("no server") },
		term:   &bytes.Buffer{},
	}
	if err := c.copy("url"); err == nil || !strings.Contains(err.Error(), "no server") {
		t.Fatalf("err = %v", err)
	}
}
