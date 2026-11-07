package system

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrepareEditor(t *testing.T) {
	t.Setenv("EDITOR", "vim --wait")
	cmd, read, err := PrepareEditor("original text")
	if err != nil {
		t.Fatal(err)
	}
	path := cmd.Args[len(cmd.Args)-1]
	if !reflect.DeepEqual(cmd.Args, []string{"vim", "--wait", path}) {
		t.Fatalf("command = %#v", cmd.Args)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "original text" {
		t.Fatalf("initial file = %q, err = %v", got, err)
	}
	if err := os.WriteFile(path, []byte("edited text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := read(); err != nil || got != "edited text" {
		t.Fatalf("read() = %q, %v", got, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary file still exists: %v", err)
	}
}

func TestPrepareEditorDefaultsToNano(t *testing.T) {
	t.Setenv("EDITOR", "")
	cmd, read, err := PrepareEditor("")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "nano" || filepath.Base(cmd.Args[len(cmd.Args)-1]) == "" {
		t.Fatalf("command = %#v", cmd.Args)
	}
	if _, err := read(); err != nil {
		t.Fatal(err)
	}
}
