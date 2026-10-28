package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDone bool
		wantErr  bool
		wantOut  string
	}{
		{"no args", nil, false, false, ""},
		{"help", []string{"--help"}, true, false, "Usage:"},
		{"short help", []string{"-h"}, true, false, "Usage:"},
		{"version", []string{"--version"}, true, false, "jt dev"},
		{"short version", []string{"-v"}, true, false, "jt dev"},
		{"unknown flag", []string{"--nope"}, true, true, ""},
		{"positional", []string{"board"}, true, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			done, err := parseArgs(tt.args, &out)
			if done != tt.wantDone || (err != nil) != tt.wantErr {
				t.Fatalf("done=%v err=%v, want done=%v err=%v", done, err, tt.wantDone, tt.wantErr)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("output %q does not contain %q", out.String(), tt.wantOut)
			}
		})
	}
}
