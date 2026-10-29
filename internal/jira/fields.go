package jira

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
)

// systemFieldOrder puts the common system fields first. Other fields follow by
// name.
var systemFieldOrder = []string{
	"summary", "description", "priority", "labels", "components", "fixVersions", "versions", "duedate", "environment",
}

type issueEditMeta struct {
	Fields   map[string]json.RawMessage `json:"fields"`
	EditMeta struct {
		Fields map[string]fieldMeta `json:"fields"`
	} `json:"editmeta"`
}

type fieldMeta struct {
	Required bool   `json:"required"`
	Name     string `json:"name"`
	Schema   struct {
		Type   string `json:"type"`
		Items  string `json:"items"`
		Custom string `json:"custom"`
	} `json:"schema"`
	Operations    []string `json:"operations"`
	AllowedValues []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		Archived bool   `json:"archived"`
	} `json:"allowedValues"`
}

// fieldsKey identifies the issues that share field definitions. Workflow
// statuses can lock fields, so the status is part of it.
type fieldsKey struct {
	project, issueType, statusID string
}

// EditFields lists the fields that can be edited on an issue, with their
// current values. Fields jt has no input for are left out. The definitions are
// fetched once per project, issue type and status; later calls only fetch the
// values. The v2 API is used on every installation, as jira-cli's own edit
// does, so rich text comes back as wiki markup rather than ADF.
func (c *Client) EditFields(iss Issue) ([]EditField, error) {
	project, _, _ := strings.Cut(iss.Key, "-")
	k := fieldsKey{project, iss.Type, iss.StatusID}
	cacheable := iss.Type != "" && iss.StatusID != ""

	c.mu.Lock()
	meta, cached := c.fieldMeta[k]
	c.mu.Unlock()

	query := "?expand=editmeta"
	if cached {
		if len(meta) == 0 {
			return nil, nil
		}
		query = "?fields=" + url.QueryEscape(strings.Join(slices.Sorted(maps.Keys(meta)), ","))
	}
	var res issueEditMeta
	if err := c.getJSON(c.jc.GetV2, "/issue/"+url.PathEscape(iss.Key)+query, &res); err != nil {
		return nil, err
	}
	if !cached {
		meta = res.supported()
		if cacheable {
			c.mu.Lock()
			if c.fieldMeta == nil {
				c.fieldMeta = make(map[fieldsKey]map[string]fieldMeta)
			}
			c.fieldMeta[k] = meta
			c.mu.Unlock()
		}
	}
	return editFields(meta, res.Fields), nil
}

// SearchUsers finds people who can be assigned issues in the issue's project
// whose name or email match query.
func (c *Client) SearchUsers(issueKey, query string) ([]Option, error) {
	project, _, _ := strings.Cut(issueKey, "-")
	users, err := c.jc.UserSearch(&jcli.UserSearchOptions{Project: project, Query: query, MaxResults: 20})
	if err != nil {
		return nil, err
	}
	out := make([]Option, 0, len(users))
	for _, u := range users {
		if !u.Active {
			continue
		}
		id := cmp.Or(u.AccountID, u.Name)
		out = append(out, Option{ID: id, Name: cmp.Or(u.DisplayName, id)})
	}
	return out, nil
}

// Edit sets the given fields on an issue to their values.
func (c *Client) Edit(key string, fields []EditField) error {
	payload, err := editPayload(fields)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"fields": payload})
	if err != nil {
		return err
	}
	res, err := c.jc.PutV2(context.Background(), "/issue/"+url.PathEscape(key), body, jcli.Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return jcli.ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		var errs jcli.Errors
		_ = json.NewDecoder(res.Body).Decode(&errs)
		return &jcli.ErrUnexpectedResponse{Body: errs, Status: res.Status, StatusCode: res.StatusCode}
	}
	return nil
}

// supported returns the definitions of the fields jt can edit.
func (r issueEditMeta) supported() map[string]fieldMeta {
	out := make(map[string]fieldMeta)
	for id, m := range r.EditMeta.Fields {
		if _, ok := fieldKind(id, m); ok && slices.Contains(m.Operations, "set") {
			out[id] = m
		}
	}
	return out
}

// editFields builds the supported fields in meta, filled with their values.
func editFields(meta map[string]fieldMeta, values map[string]json.RawMessage) []EditField {
	var out []EditField
	for id, m := range meta {
		kind, _ := fieldKind(id, m)
		f := EditField{ID: id, Name: m.Name, Kind: kind, Required: m.Required}
		f.setValue(values[id])
		for _, v := range m.AllowedValues {
			// Archived versions can't be picked, but stay listed while chosen.
			if v.Archived && !slices.Contains(f.Chosen, v.ID) {
				continue
			}
			f.Options = append(f.Options, Option{ID: v.ID, Name: cmp.Or(v.Name, v.Value)})
		}
		if (kind == FieldSelect || kind == FieldMultiSelect) && len(f.Options) == 0 {
			continue
		}
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b EditField) int {
		return cmp.Or(cmp.Compare(fieldRank(a.ID), fieldRank(b.ID)), strings.Compare(a.Name, b.Name))
	})
	return out
}

func fieldRank(id string) int {
	if i := slices.Index(systemFieldOrder, id); i >= 0 {
		return i
	}
	return len(systemFieldOrder)
}

// fieldKind maps a field's schema to the input used for it, and reports
// whether jt has one. Selects with nothing to choose from have none.
func fieldKind(id string, m fieldMeta) (FieldKind, bool) {
	s := m.Schema
	custom := s.Custom[strings.LastIndex(s.Custom, ":")+1:]
	hasOptions := len(m.AllowedValues) > 0
	switch {
	case s.Type == "string" && (id == "summary" || custom == "textfield" || custom == "url"):
		return FieldText, true
	case s.Type == "string" && (id == "description" || id == "environment" || custom == "textarea"):
		return FieldTextArea, true
	case s.Type == "number":
		return FieldNumber, true
	case s.Type == "date":
		return FieldDate, true
	case s.Type == "datetime":
		return FieldDateTime, true
	case s.Type == "option" || s.Type == "priority":
		return FieldSelect, hasOptions
	case s.Type == "array" && (s.Items == "option" || s.Items == "component" || s.Items == "version"):
		return FieldMultiSelect, hasOptions
	case s.Type == "user":
		return FieldUser, true
	case s.Type == "array" && s.Items == "user":
		return FieldMultiUser, true
	case s.Type == "array" && s.Items == "string" && (id == "labels" || custom == "labels"):
		return FieldLabels, true
	}
	return 0, false
}

type userValue struct {
	AccountID   string `json:"accountId"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// setValue reads the field's current value from the issue. A missing or null
// value leaves the field empty.
func (f *EditField) setValue(raw json.RawMessage) {
	switch f.Kind {
	case FieldNumber:
		var n *float64
		if json.Unmarshal(raw, &n) == nil && n != nil {
			f.Text = strconv.FormatFloat(*n, 'f', -1, 64)
		}
	case FieldSelect:
		var v *struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &v) == nil && v != nil && v.ID != "" {
			f.Chosen = []string{v.ID}
		}
	case FieldMultiSelect:
		var vs []struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &vs)
		for _, v := range vs {
			f.Chosen = append(f.Chosen, v.ID)
		}
	case FieldUser, FieldMultiUser:
		var vs []userValue
		var one *userValue
		if f.Kind == FieldUser {
			if json.Unmarshal(raw, &one) == nil && one != nil {
				vs = append(vs, *one)
			}
		} else {
			_ = json.Unmarshal(raw, &vs)
		}
		for _, v := range vs {
			id := cmp.Or(v.AccountID, v.Name)
			f.Chosen = append(f.Chosen, id)
			f.Options = append(f.Options, Option{ID: id, Name: cmp.Or(v.DisplayName, id)})
		}
	case FieldLabels:
		_ = json.Unmarshal(raw, &f.Chosen)
	default:
		_ = json.Unmarshal(raw, &f.Text)
	}
}

// editPayload builds the "fields" object of an edit request. Empty values
// clear the field. People are identified by account ID.
func editPayload(fields []EditField) (map[string]any, error) {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		text := strings.TrimSpace(f.Text)
		switch f.Kind {
		case FieldNumber:
			if text == "" {
				out[f.ID] = nil
				break
			}
			n, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, fmt.Errorf("%s: %q is not a number", f.Name, f.Text)
			}
			out[f.ID] = n
		case FieldSelect:
			if len(f.Chosen) == 0 {
				out[f.ID] = nil
				break
			}
			out[f.ID] = map[string]string{"id": f.Chosen[0]}
		case FieldMultiSelect:
			ids := make([]map[string]string, len(f.Chosen))
			for i, id := range f.Chosen {
				ids[i] = map[string]string{"id": id}
			}
			out[f.ID] = ids
		case FieldUser:
			if len(f.Chosen) == 0 {
				out[f.ID] = nil
				break
			}
			out[f.ID] = userRef(f.Chosen[0])
		case FieldMultiUser:
			users := make([]map[string]string, len(f.Chosen))
			for i, id := range f.Chosen {
				users[i] = userRef(id)
			}
			out[f.ID] = users
		case FieldLabels:
			out[f.ID] = append([]string{}, f.Chosen...)
		default:
			if text == "" {
				out[f.ID] = nil
				break
			}
			out[f.ID] = f.Text
		}
	}
	return out, nil
}

func userRef(id string) map[string]string {
	return map[string]string{"accountId": id}
}
