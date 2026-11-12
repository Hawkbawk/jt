package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
)

const createFieldsJSON = `{"startAt": 0, "total": 2, "values": [
	{"fieldId": "project", "name": "Project", "required": true, "schema": {"type": "project"}},
	{"fieldId": "issuetype", "name": "Issue Type", "required": true, "schema": {"type": "issuetype"}},
	{"fieldId": "reporter", "name": "Reporter", "required": true, "schema": {"type": "user"}},
	{"fieldId": "summary", "name": "Summary", "required": true, "schema": {"type": "string"}},
	{"fieldId": "assignee", "name": "Assignee", "schema": {"type": "user"}},
	{"fieldId": "priority", "name": "Priority", "schema": {"type": "priority"}, "allowedValues": [{"id": "1", "name": "High"}]},
	{"fieldId": "customfield_6", "name": "Sprint", "schema": {"type": "array", "items": "json", "custom": "x:gh-sprint"}}
]}`

func TestCreateIssueTypesSkipsSubtasksAndPages(t *testing.T) {
	var starts []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts = append(starts, r.URL.Query().Get("startAt"))
		if r.URL.Path != "/rest/api/2/issue/createmeta/A/issuetypes" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("startAt") == "0" {
			_, _ = w.Write([]byte(`{"startAt": 0, "total": 3, "values": [{"id": "1", "name": "Bug"}, {"id": "2", "name": "Sub", "subtask": true}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"startAt": 2, "total": 3, "isLast": true, "values": [{"id": "3", "name": "Story"}]}`))
	}))
	defer ts.Close()
	c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

	got, err := c.CreateIssueTypes("A")
	if err != nil {
		t.Fatal(err)
	}
	want := []IssueType{{ID: "1", Name: "Bug"}, {ID: "3", Name: "Story"}}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(starts, []string{"0", "2"}) {
		t.Fatalf("got %+v after requests at %v", got, starts)
	}
}

func TestCreateFieldsFromCreateMetaCachesPerProjectAndType(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(createFieldsJSON))
	}))
	defer ts.Close()
	c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

	got, err := c.CreateFields("A", "1")
	if err != nil {
		t.Fatal(err)
	}
	want := []IssueField{
		{ID: "summary", Name: "Summary", Kind: FieldText, Required: true},
		{ID: "priority", Name: "Priority", Kind: FieldSelect, Options: []Option{{ID: "1", Name: "High"}}},
		{ID: "assignee", Name: "Assignee", Kind: FieldUser},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %+v, want %+v", got, want)
	}
	if _, err := c.CreateFields("A", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateFields("A", "2"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want one per project and issue type", requests)
	}
}

func TestCreateSendsFieldsAndReturnsKey(t *testing.T) {
	var body map[string]map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": "10", "key": "A-7"}`))
	}))
	defer ts.Close()
	c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

	key, err := c.Create("A", "1", []IssueField{
		{ID: "summary", Kind: FieldText, Text: "Fix it"},
		{ID: "description", Kind: FieldTextArea},
		{ID: "assignee", Kind: FieldUser, Chosen: []Option{{ID: "u1", Name: "Ada"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if key != "A-7" {
		t.Errorf("key = %q", key)
	}
	f := body["fields"]
	if f["summary"] != "Fix it" || f["project"].(map[string]any)["key"] != "A" ||
		f["issuetype"].(map[string]any)["id"] != "1" || f["assignee"].(map[string]any)["accountId"] != "u1" {
		t.Errorf("fields = %v", f)
	}
	if _, ok := f["description"]; ok {
		t.Errorf("empty fields should be left out: %v", f)
	}
}

func TestCreateReportsJiraErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors": {"summary": "Summary is required"}}`))
	}))
	defer ts.Close()
	c := &Client{jc: jcli.NewClient(jcli.Config{Server: ts.URL})}

	_, err := c.Create("A", "1", nil)
	if err == nil || !strings.Contains(err.Error(), "Summary is required") {
		t.Fatalf("err = %v", err)
	}
}
