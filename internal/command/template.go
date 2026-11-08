// Package command renders configured command arguments with issue data.
package command

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/hawkbawk/jt/internal/jira"
)

// Data is the ticket context available to command argument templates.
type Data struct {
	Key         string
	IssueKey    string
	Project     string
	Board       string
	URL         string
	Title       string
	Description string
	Comments    []jira.Comment
}

// RenderArgs renders each argument as an independent Go template.
func RenderArgs(args []string, data Data) ([]string, error) {
	rendered := make([]string, len(args))
	for i, arg := range args {
		t, err := template.New(fmt.Sprintf("argument %d", i+1)).Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("parsing argument %d: %w", i+1, err)
		}
		var out bytes.Buffer
		if err := t.Execute(&out, data); err != nil {
			return nil, fmt.Errorf("rendering argument %d: %w", i+1, err)
		}
		rendered[i] = out.String()
	}
	return rendered, nil
}
