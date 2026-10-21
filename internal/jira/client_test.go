package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"
)

func TestCLIConfigPath(t *testing.T) {
	t.Setenv("JIRA_CONFIG_FILE", "")
	if got := CLIConfigPath("/cfg"); got != "/cfg/.jira/.config.yml" {
		t.Errorf("got %s", got)
	}
	t.Setenv("JIRA_CONFIG_FILE", "/elsewhere.yml")
	if got := CLIConfigPath("/cfg"); got != "/elsewhere.yml" {
		t.Errorf("got %s", got)
	}
}

func TestIssueFieldsToIssueHandlesNullFields(t *testing.T) {
	var f issueFields
	err := json.Unmarshal([]byte(`{"key": "A-1", "fields": {
		"summary": "Do it", "status": {"id": "3", "name": "In Progress"},
		"assignee": null, "issuetype": {"name": "Bug"}, "priority": null}}`), &f)
	if err != nil {
		t.Fatal(err)
	}
	got := f.toIssue()
	want := Issue{Key: "A-1", Summary: "Do it", StatusID: "3", Status: "In Progress", Type: "Bug"}
	if got != want {
		t.Fatalf("got %+v", got)
	}
}

func TestToMarkdown(t *testing.T) {
	var doc *adf.ADF
	if err := json.Unmarshal([]byte(`{"version": 1, "type": "doc", "content": [
		{"type": "paragraph", "content": [{"type": "text", "text": "hello"}]}]}`), &doc); err != nil {
		t.Fatal(err)
	}
	if got := toMarkdown(doc); !strings.Contains(got, "hello") {
		t.Errorf("ADF: %q", got)
	}
	if got := toMarkdown("*bold*"); !strings.Contains(got, "**bold**") {
		t.Errorf("wiki: %q", got)
	}
	if toMarkdown((*adf.ADF)(nil)) != "" || toMarkdown(nil) != "" {
		t.Error("expected empty output for missing description")
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime("not a time"); got != "not a time" {
		t.Errorf("got %q", got)
	}
	if got := formatTime("2026-10-14T15:00:51.588-0600"); len(got) != len(displayLayout) {
		t.Errorf("got %q", got)
	}
}
