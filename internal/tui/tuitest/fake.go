// Package tuitest holds the fakes and helpers the screen tests share. It doesn't
// import the screens, so their tests can use it without an import cycle.
package tuitest

import (
	"errors"

	"github.com/hawkbawk/jt/internal/jira"
)

// Backend is an in-memory fake of the Jira functionality the screens use. It
// records calls in its exported fields.
type Backend struct {
	Project        string
	DefaultBoard   int
	ProjectList    []jira.Project
	BoardData      *jira.Board
	BoardErr       error
	Detail         *jira.IssueDetail
	CommentList    *jira.CommentList
	TransitionList []jira.Transition
	MoveErr        error
	Moved          []string
	AssignErr      error
	Assigned       []string
	Commented      []string
	CommentErr     error
	BoardCalls     []string
	BoardIDCalls   []int
	BoardList      []jira.BoardSummary
	FilterCalls    []string
	EditFieldList  []jira.IssueField
	EditErr        error
	Edited         [][]jira.IssueField
	IssueTypeList  []string
	Found          []jira.Issue
	FindErr        error
	FindCalls      []string
	Users          []jira.Option
	UserErr        error
	UserQueries    []string
}

// DefaultProject implements the screens' Backend.
func (f *Backend) DefaultProject() string { return f.Project }

// SetDefaultProject implements the screens' Backend.
func (f *Backend) SetDefaultProject(project string) { f.Project = project }

// SetDefaultBoard implements the screens' Backend.
func (f *Backend) SetDefaultBoard(id int) { f.DefaultBoard = id }

// Projects implements the screens' Backend.
func (f *Backend) Projects() ([]jira.Project, error) { return f.ProjectList, nil }

// IssueTypes implements the screens' Backend.
func (f *Backend) IssueTypes() ([]string, error) { return f.IssueTypeList, nil }

// Search implements the screens' Backend.
func (f *Backend) Search(jql string) ([]jira.Issue, error) {
	f.FindCalls = append(f.FindCalls, jql)
	return f.Found, f.FindErr
}

// Boards implements the screens' Backend.
func (f *Backend) Boards(string) ([]jira.BoardSummary, error) { return f.BoardList, nil }

// Board implements the screens' Backend.
func (f *Backend) Board(project string, boardID int, filterJQL string) (*jira.Board, error) {
	f.BoardCalls = append(f.BoardCalls, project)
	f.BoardIDCalls = append(f.BoardIDCalls, boardID)
	f.FilterCalls = append(f.FilterCalls, filterJQL)
	if f.BoardErr != nil {
		return nil, f.BoardErr
	}
	b := *f.BoardData
	b.Project = project
	return &b, nil
}

// Issue implements the screens' Backend.
func (f *Backend) Issue(string) (*jira.IssueDetail, error) { return f.Detail, nil }

// Comments implements the screens' Backend.
func (f *Backend) Comments(string) (*jira.CommentList, error) { return f.CommentList, nil }

// Attachment implements the screens' Backend.
func (f *Backend) Attachment(string) ([]byte, error) { return nil, errors.New("no attachments") }

// Transitions implements the screens' Backend.
func (f *Backend) Transitions(jira.Issue) ([]jira.Transition, error) {
	return f.TransitionList, nil
}

// Transition implements the screens' Backend.
func (f *Backend) Transition(key, id string) error {
	if f.MoveErr != nil {
		return f.MoveErr
	}
	f.Moved = append(f.Moved, key+":"+id)
	return nil
}

// Assign implements the screens' Backend.
func (f *Backend) Assign(key, id string) error {
	if f.AssignErr != nil {
		return f.AssignErr
	}
	f.Assigned = append(f.Assigned, key+":"+id)
	return nil
}

// AddComment implements the screens' Backend.
func (f *Backend) AddComment(key, body string) error {
	if f.CommentErr != nil {
		return f.CommentErr
	}
	f.Commented = append(f.Commented, key+":"+body)
	return nil
}

// EditFields implements the screens' Backend.
func (f *Backend) EditFields(jira.Issue) ([]jira.IssueField, error) {
	return f.EditFieldList, nil
}

// Edit implements the screens' Backend.
func (f *Backend) Edit(_ string, fields []jira.IssueField) error {
	if f.EditErr != nil {
		return f.EditErr
	}
	f.Edited = append(f.Edited, fields)
	return nil
}

// SearchUsers implements the screens' Backend.
func (f *Backend) SearchUsers(_, query string) ([]jira.Option, error) {
	f.UserQueries = append(f.UserQueries, query)
	return f.Users, f.UserErr
}

// IssueURL implements the screens' Backend.
func (f *Backend) IssueURL(key string) string {
	return "https://jira.example.com/browse/" + key
}

// Settings is an in-memory fake of the screens' settings. It records what was
// saved.
type Settings struct {
	Saved    []string
	Boards   []int
	Projects []string
	Err      error
}

// SaveProject implements the screens' Settings.
func (f *Settings) SaveProject(key string) error {
	f.Projects = append(f.Projects, key)
	return f.Err
}

// SaveBoard implements the screens' Settings.
func (f *Settings) SaveBoard(id int) error {
	f.Boards = append(f.Boards, id)
	return f.Err
}

// SaveTheme implements the screens' Settings.
func (f *Settings) SaveTheme(id string) error {
	f.Saved = append(f.Saved, id)
	return f.Err
}
