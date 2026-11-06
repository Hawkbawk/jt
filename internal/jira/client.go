package jira

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"
	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/jira/filter/issue"
	"github.com/ankitpokhrel/jira-cli/pkg/md"

	"github.com/hawkbawk/jt/internal/config"
)

const (
	timeout           = 15 * time.Second
	pageSize          = 100
	boardIssuesLimit  = 1000
	searchIssuesLimit = 100
	commentsLimit     = 100

	jiraTimeLayout = "2006-01-02T15:04:05.000-0700"
	displayLayout  = "2006-01-02 15:04"
)

// Client talks to Jira using jira-cli's HTTP client.
type Client struct {
	jc  *jcli.Client
	cfg config.Jira

	mu        sync.Mutex
	quick     map[int][]Filter
	fieldMeta map[fieldsKey]map[string]fieldMeta
	moves     map[fieldsKey][]Transition
}

// New builds a client from jt's Jira settings and the API token. It signs in
// with basic auth, jira-cli's default.
func New(cfg config.Jira, token string) *Client {
	jc := jcli.NewClient(jcli.Config{
		Server:   cfg.Server,
		Login:    cfg.Login,
		APIToken: token,
	}, jcli.WithTimeout(timeout))
	return &Client{jc: jc, cfg: cfg}
}

// DefaultProject returns the project key set in jt's settings, if any.
func (c *Client) DefaultProject() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.Project
}

// SetDefaultProject changes the default project. The default board belongs to
// the old project, so it's dropped when the project changes.
func (c *Client) SetDefaultProject(project string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Project != project {
		c.cfg.BoardID = 0
	}
	c.cfg.Project = project
}

// SetDefaultBoard changes the board opened for the default project.
func (c *Client) SetDefaultBoard(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.BoardID = id
}

// Projects lists the projects visible to the user, sorted by key.
func (c *Client) Projects() ([]Project, error) {
	ps, err := c.jc.Project()
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(ps))
	for _, p := range ps {
		out = append(out, Project{Key: p.Key, Name: p.Name})
	}
	slices.SortFunc(out, func(a, b Project) int { return strings.Compare(a.Key, b.Key) })
	return out, nil
}

// IssueTypes lists the names of the issue types on the site, sorted.
func (c *Client) IssueTypes() ([]string, error) {
	var res []struct {
		Name string `json:"name"`
	}
	if err := c.getJSON(c.jc.Get, "/issuetype", &res); err != nil {
		return nil, err
	}
	var out []string
	for _, t := range res {
		out = append(out, t.Name)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// CheckLogin checks that Jira accepts the login and token.
func (c *Client) CheckLogin() error {
	_, err := c.jc.Me()
	return err
}

// Boards lists a project's boards.
func (c *Client) Boards(project string) ([]BoardSummary, error) {
	boards, err := c.boards(project)
	if err != nil {
		return nil, err
	}
	out := make([]BoardSummary, len(boards))
	for i, b := range boards {
		out[i] = BoardSummary{ID: b.ID, Name: b.Name, Type: b.Type}
	}
	return out, nil
}

func (c *Client) boards(project string) ([]boardInfo, error) {
	var res struct {
		Values []boardInfo `json:"values"`
	}
	if err := c.getJSON(c.jc.GetV1, "/board?projectKeyOrId="+url.QueryEscape(project), &res); err != nil {
		return nil, err
	}
	return res.Values, nil
}

// Board loads a board for a project. boardID picks the board when it's not
// zero. Otherwise the configured board is used when it belongs to the project,
// and failing that one is chosen from the project's boards. filterJQL, when
// set, narrows the issues loaded.
func (c *Client) Board(project string, boardID int, filterJQL string) (*Board, error) {
	c.mu.Lock()
	if boardID == 0 && c.cfg.Project == project {
		boardID = c.cfg.BoardID
	}
	c.mu.Unlock()

	var info boardInfo
	if boardID != 0 {
		if err := c.getJSON(c.jc.GetV1, fmt.Sprintf("/board/%d", boardID), &info); err != nil {
			return nil, err
		}
	} else {
		boards, err := c.boards(project)
		if err != nil {
			return nil, err
		}
		if info, err = pickBoard(project, boards); err != nil {
			return nil, err
		}
	}

	var cfg boardConfig
	if err := c.getJSON(c.jc.GetV1, fmt.Sprintf("/board/%d/configuration", info.ID), &cfg); err != nil {
		return nil, err
	}
	cols, err := columnsFrom(cfg)
	if err != nil {
		return nil, err
	}

	issues, err := c.boardIssues(info.ID, combineJQL(boardJQL(info.Type), filterJQL))
	if err != nil {
		return nil, err
	}
	distribute(cols, issues)

	quick := c.quickFilters(info.ID)

	return &Board{
		ID: info.ID, Name: info.Name, Type: info.Type, Project: project, Columns: cols,
		QuickFilters: quick, Assignees: AssigneeFilters(issues),
	}, nil
}

// quickFilters returns a board's quick filters, fetching them once per board
// since they rarely change. They're optional, so a failed lookup yields none
// and isn't cached.
func (c *Client) quickFilters(boardID int) []Filter {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.quick[boardID]
	if ok {
		return cached
	}

	var qf quickFilterList
	if err := c.getJSON(c.jc.GetV1, fmt.Sprintf("/board/%d/quickfilter", boardID), &qf); err != nil {
		return nil
	}
	quick := quickFilters(qf)

	if c.quick == nil {
		c.quick = make(map[int][]Filter)
	}
	c.quick[boardID] = quick
	return quick
}

type issueFields struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
			AccountID   string `json:"accountId"`
			Name        string `json:"name"`
		} `json:"assignee"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
	} `json:"fields"`
}

func (f issueFields) toIssue() Issue {
	iss := Issue{
		Key:      f.Key,
		Summary:  f.Fields.Summary,
		StatusID: f.Fields.Status.ID,
		Status:   f.Fields.Status.Name,
		Type:     f.Fields.IssueType.Name,
	}
	if f.Fields.Assignee != nil {
		a := f.Fields.Assignee
		iss.Assignee = Option{ID: cmp.Or(a.AccountID, a.Name), Name: a.DisplayName}
	}
	if f.Fields.Priority != nil {
		iss.Priority = f.Fields.Priority.Name
	}
	return iss
}

func (c *Client) boardIssues(boardID int, jql string) ([]Issue, error) {
	var out []Issue
	for start := 0; start < boardIssuesLimit; start += pageSize {
		path := fmt.Sprintf("/board/%d/issue?jql=%s&fields=summary,status,assignee,issuetype,priority&startAt=%d&maxResults=%d",
			boardID, url.QueryEscape(jql), start, pageSize)
		var page struct {
			Total  int           `json:"total"`
			Issues []issueFields `json:"issues"`
		}
		if err := c.getJSON(c.jc.GetV1, path, &page); err != nil {
			return nil, err
		}
		for _, f := range page.Issues {
			out = append(out, f.toIssue())
		}
		if len(page.Issues) == 0 || start+len(page.Issues) >= page.Total {
			break
		}
	}
	return out, nil
}

// Search returns the issues matching jql, up to a limit, in the order the JQL
// asks for.
func (c *Client) Search(jql string) ([]Issue, error) {
	var out []Issue
	token := ""
	for len(out) < searchIssuesLimit {
		path := fmt.Sprintf("/search/jql?jql=%s&fields=summary,status,assignee,issuetype,priority&maxResults=%d",
			url.QueryEscape(jql), min(pageSize, searchIssuesLimit-len(out)))
		if token != "" {
			path += "&nextPageToken=" + url.QueryEscape(token)
		}
		var page struct {
			Issues        []issueFields `json:"issues"`
			NextPageToken string        `json:"nextPageToken"`
		}
		if err := c.getJSON(c.jc.Get, path, &page); err != nil {
			return nil, err
		}
		for _, f := range page.Issues {
			out = append(out, f.toIssue())
		}
		if len(page.Issues) == 0 || page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return out, nil
}

// Issue fetches an issue with its description.
func (c *Client) Issue(key string) (*IssueDetail, error) {
	iss, err := c.jc.GetIssue(key, issue.NewNumCommentsFilter(0))
	if err != nil {
		return nil, err
	}
	f := iss.Fields
	d := &IssueDetail{
		Key:         iss.Key,
		Summary:     f.Summary,
		Status:      f.Status.Name,
		Assignee:    Option{Name: f.Assignee.Name},
		Type:        f.IssueType.Name,
		Priority:    f.Priority.Name,
		Description: toMarkdown(f.Description),
		Reporter:    f.Reporter.Name,
		Labels:      f.Labels,
		Created:     formatTime(f.Created),
		Updated:     formatTime(f.Updated),
	}
	return d, nil
}

// Comments fetches the newest comments on an issue, newest first, along with
// how many it has in all.
func (c *Client) Comments(key string) (*CommentList, error) {
	path := fmt.Sprintf("/issue/%s/comment?orderBy=-created&maxResults=%d", url.PathEscape(key), commentsLimit)
	var res struct {
		Comments []struct {
			Author struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
			Body    *adf.ADF `json:"body"`
			Created string   `json:"created"`
			Updated string   `json:"updated"`
		} `json:"comments"`
		Total int `json:"total"`
	}
	if err := c.getJSON(c.jc.Get, path, &res); err != nil {
		return nil, err
	}
	out := make([]Comment, 0, len(res.Comments))
	for _, cm := range res.Comments {
		out = append(out, Comment{
			Author:  cm.Author.DisplayName,
			Created: formatTime(cm.Created),
			Edited:  editedAfter(cm.Created, cm.Updated),
			Body:    toMarkdown(cm.Body),
		})
	}
	return &CommentList{Items: out, Total: res.Total}, nil
}

// Transitions lists the transitions available for an issue. They're fetched
// once per project, issue type and status.
func (c *Client) Transitions(iss Issue) ([]Transition, error) {
	project, _, _ := strings.Cut(iss.Key, "-")
	k := fieldsKey{project, iss.Type, iss.StatusID}
	cacheable := iss.Type != "" && iss.StatusID != ""
	if cacheable {
		c.mu.Lock()
		cached, ok := c.moves[k]
		c.mu.Unlock()
		if ok {
			return slices.Clone(cached), nil
		}
	}
	var res struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := c.getJSON(c.jc.Get, "/issue/"+url.PathEscape(iss.Key)+"/transitions", &res); err != nil {
		return nil, err
	}
	out := make([]Transition, 0, len(res.Transitions))
	for _, t := range res.Transitions {
		out = append(out, Transition{ID: t.ID, Name: t.Name, To: t.To.Name})
	}
	if cacheable {
		c.mu.Lock()
		if c.moves == nil {
			c.moves = make(map[fieldsKey][]Transition)
		}
		c.moves[k] = slices.Clone(out)
		c.mu.Unlock()
	}
	return out, nil
}

// Transition moves an issue through the given transition.
func (c *Client) Transition(key, transitionID string) error {
	_, err := c.jc.Transition(key, &jcli.TransitionRequest{
		Transition: &jcli.TransitionRequestData{ID: transitionID},
	})
	return err
}

// Assign sets an issue's assignee. An empty assigneeID unassigns it.
func (c *Client) Assign(key, assigneeID string) error {
	if assigneeID == "" {
		assigneeID = jcli.AssigneeNone
	}
	return c.jc.AssignIssue(key, assigneeID)
}

// AddComment posts a new comment on an issue.
func (c *Client) AddComment(key, body string) error {
	return c.jc.AddIssueComment(key, body, false)
}

// IssueURL returns the browser URL for an issue on the configured server.
func (c *Client) IssueURL(key string) string {
	return c.cfg.Server + "/browse/" + url.PathEscape(key)
}

type getFunc func(ctx context.Context, path string, headers jcli.Header) (*http.Response, error)

func (c *Client) getJSON(get getFunc, path string, out any) error {
	res, err := get(context.Background(), path, nil)
	if err != nil {
		return err
	}
	if res == nil {
		return jcli.ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		var body jcli.Errors
		_ = json.NewDecoder(res.Body).Decode(&body)
		return responseError(res, body)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// responseError turns a failed response into a one-line error. jira-cli's
// ErrUnexpectedResponse renders as a multi-line "Error:" block, which reads
// badly once the UI adds its own prefix.
func responseError(res *http.Response, body jcli.Errors) error {
	msgs := slices.Clone(body.ErrorMessages)
	for _, k := range slices.Sorted(maps.Keys(body.Errors)) {
		msgs = append(msgs, k+": "+body.Errors[k])
	}
	if len(msgs) == 0 {
		return errors.New(res.Status)
	}
	return errors.New(strings.Join(msgs, "; "))
}

// toMarkdown converts an ADF document (cloud) or Jira wiki markup (server) to markdown.
func toMarkdown(v any) string {
	switch v := v.(type) {
	case *adf.ADF:
		if v == nil {
			return ""
		}
		return adf.NewTranslator(v, adf.NewMarkdownTranslator()).Translate()
	case string:
		return md.FromJiraMD(v)
	default:
		return ""
	}
}

// editedAfter reports whether updated is later than created by more than the
// small gap Jira leaves on comments that were never edited.
func editedAfter(created, updated string) bool {
	c, err := time.Parse(jiraTimeLayout, created)
	if err != nil {
		return false
	}
	u, err := time.Parse(jiraTimeLayout, updated)
	if err != nil {
		return false
	}
	return u.Sub(c) > time.Second
}

func formatTime(s string) string {
	t, err := time.Parse(jiraTimeLayout, s)
	if err != nil {
		return s
	}
	return t.Local().Format(displayLayout)
}
