package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
)

const editMetaJSON = `{
	"fields": {
		"summary": "Fix the thing",
		"description": "h1. Steps",
		"priority": {"id": "3", "name": "Medium"},
		"labels": ["backend"],
		"fixVersions": [{"id": "v1", "name": "1.0"}],
		"customfield_1": 3.0,
		"customfield_2": {"id": "o2", "value": "Team B"},
		"customfield_3": null,
		"customfield_7": {"accountId": "u1", "displayName": "Ada"},
		"customfield_8": [{"accountId": "u2", "displayName": "Bo"}, {"accountId": "u3", "displayName": "Cy"}]
	},
	"editmeta": {"fields": {
		"customfield_2": {"name": "Sprint Team", "operations": ["set"], "schema": {"type": "option", "custom": "x:select"},
			"allowedValues": [{"id": "o1", "value": "Team A"}, {"id": "o2", "value": "Team B"}]},
		"customfield_1": {"name": "Story Points", "operations": ["set"], "schema": {"type": "number", "custom": "x:float"}},
		"customfield_3": {"name": "Notes", "operations": ["set"], "schema": {"type": "string", "custom": "x:textarea"}},
		"customfield_7": {"name": "Owner", "operations": ["set"], "schema": {"type": "user", "custom": "x:userpicker"}},
		"customfield_8": {"name": "Reviewer(s)", "operations": ["add", "set", "remove"], "schema": {"type": "array", "items": "user", "custom": "x:multiuserpicker"}},
		"customfield_9": {"name": "Watchers", "operations": ["add", "remove"], "schema": {"type": "array", "items": "user"}},
		"customfield_4": {"name": "Banner", "operations": ["set"], "schema": {"type": "string", "custom": "x:message"}},
		"customfield_5": {"name": "A11y Type", "operations": ["set"], "schema": {"type": "option-with-child", "custom": "x:cascadingselect"},
			"allowedValues": [{"id": "c1", "value": "Visual"}]},
		"customfield_6": {"name": "Sprint", "operations": ["set"], "schema": {"type": "array", "items": "json", "custom": "x:gh-sprint"}},
		"issuetype": {"name": "Issue Type", "operations": [], "schema": {"type": "issuetype"}, "allowedValues": [{"id": "1", "name": "Bug"}]},
		"components": {"name": "Components", "operations": ["add", "set", "remove"], "schema": {"type": "array", "items": "component"}, "allowedValues": []},
		"fixVersions": {"name": "Fix versions", "operations": ["set", "add", "remove"], "schema": {"type": "array", "items": "version"},
			"allowedValues": [{"id": "v0", "name": "0.9", "archived": true}, {"id": "v1", "name": "1.0", "archived": true}, {"id": "v2", "name": "2.0"}]},
		"labels": {"name": "Labels", "operations": ["add", "set", "remove"], "schema": {"type": "array", "items": "string"}},
		"priority": {"name": "Priority", "operations": ["set"], "schema": {"type": "priority"},
			"allowedValues": [{"id": "1", "name": "High"}, {"id": "3", "name": "Medium"}]},
		"description": {"name": "Description", "operations": ["set"], "schema": {"type": "string"}},
		"summary": {"name": "Summary", "required": true, "operations": ["set"], "schema": {"type": "string"}}
	}}
}`

func TestEditFieldsFromEditMeta(t *testing.T) {
	var r issueEditMeta
	if err := json.Unmarshal([]byte(editMetaJSON), &r); err != nil {
		t.Fatal(err)
	}
	want := []EditField{
		{ID: "summary", Name: "Summary", Kind: FieldText, Required: true, Text: "Fix the thing"},
		{ID: "description", Name: "Description", Kind: FieldTextArea, Text: "h1. Steps"},
		{
			ID: "priority", Name: "Priority", Kind: FieldSelect, Chosen: []string{"3"},
			Options: []Option{{ID: "1", Name: "High"}, {ID: "3", Name: "Medium"}},
		},
		{ID: "labels", Name: "Labels", Kind: FieldLabels, Chosen: []string{"backend"}},
		{
			ID: "fixVersions", Name: "Fix versions", Kind: FieldMultiSelect, Chosen: []string{"v1"},
			Options: []Option{{ID: "v1", Name: "1.0"}, {ID: "v2", Name: "2.0"}},
		},
		{ID: "customfield_3", Name: "Notes", Kind: FieldTextArea},
		{
			ID: "customfield_7", Name: "Owner", Kind: FieldUser, Chosen: []string{"u1"},
			Options: []Option{{ID: "u1", Name: "Ada"}},
		},
		{
			ID: "customfield_8", Name: "Reviewer(s)", Kind: FieldMultiUser, Chosen: []string{"u2", "u3"},
			Options: []Option{{ID: "u2", Name: "Bo"}, {ID: "u3", Name: "Cy"}},
		},
		{
			ID: "customfield_2", Name: "Sprint Team", Kind: FieldSelect, Chosen: []string{"o2"},
			Options: []Option{{ID: "o1", Name: "Team A"}, {ID: "o2", Name: "Team B"}},
		},
		{ID: "customfield_1", Name: "Story Points", Kind: FieldNumber, Text: "3"},
	}
	if got := editFields(r.supported(), r.Fields); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestEditPayload(t *testing.T) {
	got, err := editPayload([]EditField{
		{ID: "summary", Kind: FieldText, Text: "New"},
		{ID: "description", Kind: FieldTextArea, Text: "  "},
		{ID: "priority", Kind: FieldSelect, Chosen: []string{"1"}},
		{ID: "cf_team", Kind: FieldSelect},
		{ID: "components", Kind: FieldMultiSelect, Chosen: []string{"c1", "c2"}},
		{ID: "labels", Kind: FieldLabels},
		{ID: "cf_points", Kind: FieldNumber, Text: " 5.5 "},
		{ID: "duedate", Kind: FieldDate, Text: "2026-11-01"},
		{ID: "cf_owner", Kind: FieldUser, Chosen: []string{"u1"}},
		{ID: "cf_nobody", Kind: FieldUser},
		{ID: "cf_reviewers", Kind: FieldMultiUser, Chosen: []string{"u2", "u3"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(got)
	want := `{"cf_nobody":null,"cf_owner":{"accountId":"u1"},"cf_points":5.5,` +
		`"cf_reviewers":[{"accountId":"u2"},{"accountId":"u3"}],"cf_team":null,"components":[{"id":"c1"},{"id":"c2"}],"description":null,` +
		`"duedate":"2026-11-01","labels":[],"priority":{"id":"1"},"summary":"New"}`
	if string(body) != want {
		t.Fatalf("got  %s\nwant %s", body, want)
	}
}

func TestEditPayloadRejectsBadNumber(t *testing.T) {
	if _, err := editPayload([]EditField{{ID: "cf", Name: "Story Points", Kind: FieldNumber, Text: "lots"}}); err == nil {
		t.Fatal("expected an error for a non-numeric value")
	}
}

func TestEditFieldsCachesDefinitionsPerTypeAndStatus(t *testing.T) {
	var requests []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("expand") {
			requests = append(requests, "editmeta")
			_, _ = w.Write([]byte(editMetaJSON))
			return
		}
		requests = append(requests, "values:"+r.URL.Query().Get("fields"))
		_, _ = w.Write([]byte(`{"fields": {"summary": "Other", "customfield_1": 8}}`))
	}))
	defer ts.Close()
	c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

	bug := Issue{Key: "A-1", Type: "Bug", StatusID: "1"}
	first, err := c.EditFields(bug)
	if err != nil {
		t.Fatal(err)
	}
	bug.Key = "A-2"
	second, err := c.EditFields(bug)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != len(first) || second[0].Text != "Other" || second[len(second)-1].Text != "8" {
		t.Fatalf("cached definitions should be filled with the new issue's values: %+v", second)
	}
	for _, iss := range []Issue{
		{Key: "A-3", Type: "Bug", StatusID: "2"},
		{Key: "B-1", Type: "Bug", StatusID: "1"},
		{Key: "A-4", Type: "Bug"},
		{Key: "A-4", Type: "Bug"},
	} {
		if _, err := c.EditFields(iss); err != nil {
			t.Fatal(err)
		}
	}

	if len(requests) != 6 || !strings.HasPrefix(requests[1], "values:") {
		t.Fatalf("requests = %v", requests)
	}
	for i, r := range requests {
		if i != 1 && r != "editmeta" {
			t.Errorf("request %d should fetch definitions, got %s", i, r)
		}
	}
	if ids := strings.Split(strings.TrimPrefix(requests[1], "values:"), ","); len(ids) != len(first) {
		t.Errorf("values request should ask for just the editable fields, got %v", ids)
	}
}

func TestTransitionsCachedPerTypeAndStatus(t *testing.T) {
	cached := Issue{Key: "A-1", Type: "Bug", StatusID: "1"}
	tests := []struct {
		name        string
		cached      Issue
		issue       Issue
		wantRequest bool
	}{
		{"same project, type and status", cached, Issue{Key: "A-2", Type: "Bug", StatusID: "1"}, false},
		{"different status", cached, Issue{Key: "A-2", Type: "Bug", StatusID: "2"}, true},
		{"different project", cached, Issue{Key: "B-1", Type: "Bug", StatusID: "1"}, true},
		{"different issue type", cached, Issue{Key: "A-2", Type: "Story", StatusID: "1"}, true},
		{"nothing cached", Issue{}, cached, true},
		{"no status is never cached", Issue{Key: "A-2", Type: "Bug"}, Issue{Key: "A-2", Type: "Bug"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				_, _ = w.Write([]byte(`{"transitions": [{"id": "11", "name": "Start", "to": {"name": "Doing"}}]}`))
			}))
			defer ts.Close()
			c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

			if tt.cached.Key != "" {
				if _, err := c.Transitions(tt.cached); err != nil {
					t.Fatal(err)
				}
			}
			requests = 0
			got, err := c.Transitions(tt.issue)
			if err != nil || len(got) != 1 || got[0].To != "Doing" {
				t.Fatalf("Transitions(%v) = %v, %v", tt.issue, got, err)
			}
			if made := requests > 0; made != tt.wantRequest {
				t.Errorf("made request = %v, want %v", made, tt.wantRequest)
			}
		})
	}
}
