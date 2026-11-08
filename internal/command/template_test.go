package command

import (
	"reflect"
	"testing"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestRenderArgs(t *testing.T) {
	args, err := RenderArgs([]string{
		"{{.Key}}",
		"{{.IssueKey}}",
		"{{.Project}}/{{.Board}}",
		"{{.URL}}",
		"{{.Title}}",
		"{{.Description}}",
		"{{range .Comments}}{{.Author}}: {{.Body}}{{end}}",
	}, Data{
		Key: "ABC-1", IssueKey: "ABC-1", Project: "ABC", Board: "Team",
		URL: "https://jira.example/browse/ABC-1", Title: "Title",
		Description: "details", Comments: []jira.Comment{{Author: "Ada", Body: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ABC-1", "ABC-1", "ABC/Team", "https://jira.example/browse/ABC-1", "Title", "details", "Ada: hello"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("arguments = %#v, want %#v", args, want)
	}
}

func TestRenderArgsKeepsEachArgumentSeparate(t *testing.T) {
	args, err := RenderArgs([]string{"{{.Key}}; echo unsafe", "{{.Key}}"}, Data{Key: "ABC-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ABC-1; echo unsafe", "ABC-1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("arguments = %#v, want %#v", args, want)
	}
}

func TestRenderArgsReportsInvalidTemplate(t *testing.T) {
	if _, err := RenderArgs([]string{"{{"}, Data{}); err == nil {
		t.Fatal("expected invalid template error")
	}
}
