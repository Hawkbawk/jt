package system

import (
	"os"
	"os/exec"
	"strings"
)

// PrepareEditor creates a temporary file with text and a command for $EDITOR.
// The returned function reads and removes the file after the editor exits.
func PrepareEditor(text string) (*exec.Cmd, func() (string, error), error) {
	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) == 0 {
		editor = []string{"nano"}
	}

	file, err := os.CreateTemp("", "jt-*.md")
	if err != nil {
		return nil, nil, err
	}
	name := file.Name()
	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()
		_ = os.Remove(name)
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return nil, nil, err
	}

	args := append(editor[1:], name)
	cmd := exec.Command(editor[0], args...)
	read := func() (string, error) {
		defer os.Remove(name)
		content, err := os.ReadFile(name)
		return string(content), err
	}
	return cmd, read, nil
}
