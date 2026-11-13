package jira

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
)

// createPageSize is how many items to ask for from the paginated create
// metadata endpoints.
const createPageSize = 100

// createOnly are the fields the create form never lists. The project and issue
// type are chosen first. Jira sets the reporter to whoever creates the issue.
var createOnly = []string{"project", "issuetype", "reporter"}

type createPage struct {
	StartAt    int  `json:"startAt"`
	MaxResults int  `json:"maxResults"`
	Total      int  `json:"total"`
	IsLast     bool `json:"isLast"`
}

func (p createPage) done(got int) bool {
	return got == 0 || p.IsLast || p.StartAt+got >= p.Total
}

type createTypesPage struct {
	createPage
	IssueTypes []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Subtask bool   `json:"subtask"`
	} `json:"issueTypes"`
}

type createFieldsPage struct {
	createPage
	Fields []struct {
		FieldID string `json:"fieldId"`
		fieldMeta
	} `json:"fields"`
}

// CreateIssueTypes lists the issue types that can be created in a project.
// Subtasks are left out, as they need a parent.
func (c *Client) CreateIssueTypes(project string) ([]IssueType, error) {
	var out []IssueType
	for start := 0; ; {
		var res createTypesPage
		path := "/issue/createmeta/" + url.PathEscape(project) + "/issuetypes?startAt=" + strconv.Itoa(start) +
			"&maxResults=" + strconv.Itoa(createPageSize)
		if err := c.getJSON(c.jc.GetV2, path, &res); err != nil {
			return nil, err
		}
		for _, t := range res.IssueTypes {
			if !t.Subtask {
				out = append(out, IssueType{ID: t.ID, Name: t.Name})
			}
		}
		if res.done(len(res.IssueTypes)) {
			return out, nil
		}
		start += len(res.IssueTypes)
	}
}

// CreateFields lists the fields an issue of a type can be created with. Fields
// jt has no input for are left out, as are the ones the create form handles
// itself. The definitions are fetched once per project and issue type.
func (c *Client) CreateFields(project, issueTypeID string) ([]IssueField, error) {
	k := fieldsKey{project: project, issueType: issueTypeID}
	c.mu.Lock()
	cached, ok := c.createFields[k]
	c.mu.Unlock()
	if ok {
		return slices.Clone(cached), nil
	}

	meta := make(map[string]fieldMeta)
	for start := 0; ; {
		var res createFieldsPage
		path := "/issue/createmeta/" + url.PathEscape(project) + "/issuetypes/" + url.PathEscape(issueTypeID) +
			"?startAt=" + strconv.Itoa(start) + "&maxResults=" + strconv.Itoa(createPageSize)
		if err := c.getJSON(c.jc.GetV2, path, &res); err != nil {
			return nil, err
		}
		for _, f := range res.Fields {
			if _, ok := fieldKind(f.FieldID, f.fieldMeta); ok && !slices.Contains(createOnly, f.FieldID) {
				meta[f.FieldID] = f.fieldMeta
			}
		}
		if res.done(len(res.Fields)) {
			break
		}
		start += len(res.Fields)
	}
	fields := editFields(meta, nil)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.createFields == nil {
		c.createFields = make(map[fieldsKey][]IssueField)
	}
	c.createFields[k] = fields
	return slices.Clone(fields), nil
}

// Create makes an issue of a type in a project with the given fields and
// returns its key. Fields with no value are left out.
func (c *Client) Create(project, issueTypeID string, fields []IssueField) (string, error) {
	payload, err := editPayload(fields)
	if err != nil {
		return "", err
	}
	maps.DeleteFunc(payload, func(_ string, v any) bool { return v == nil })
	payload["project"] = map[string]string{"key": project}
	payload["issuetype"] = map[string]string{"id": issueTypeID}
	body, err := json.Marshal(map[string]any{"fields": payload})
	if err != nil {
		return "", err
	}
	res, err := c.jc.PostV2(context.Background(), "/issue", body, jcli.Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", jcli.ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusCreated {
		var errs jcli.Errors
		_ = json.NewDecoder(res.Body).Decode(&errs)
		return "", responseError(res, errs)
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", err
	}
	if created.Key == "" {
		return "", errors.New("the new issue has no key in the response")
	}
	return created.Key, nil
}
