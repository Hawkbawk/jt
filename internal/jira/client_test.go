package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"

	"github.com/hawkbawk/jt/internal/config"
)

func TestSearchCancellationReachesRequest(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer ts.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c := New(config.Jira{Server: ts.URL}, "")
	done := make(chan error, 1)
	go func() {
		_, err := c.Search(ctx, "project = DEMO")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Search returned %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Search did not stop after cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("HTTP request was not canceled")
	}
}

func TestNewUsesSettingsAndToken(t *testing.T) {
	var gotPath, gotUser, gotPass string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser, gotPass, _ = r.BasicAuth()
		_, _ = w.Write([]byte(`{"key": "A-1", "fields": {}}`))
	}))
	defer ts.Close()

	c := New(config.Jira{Server: ts.URL, Login: "me@example.com", Project: "A", BoardID: 3}, "secret")
	if _, err := c.Issue("A-1"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/3/issue/A-1" || gotUser != "me@example.com" || gotPass != "secret" {
		t.Errorf("path %q, auth %q:%q", gotPath, gotUser, gotPass)
	}
	if c.DefaultProject() != "A" || c.IssueURL("A-1") != ts.URL+"/browse/A-1" {
		t.Errorf("project %q, url %q", c.DefaultProject(), c.IssueURL("A-1"))
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
