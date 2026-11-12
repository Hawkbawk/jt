// Package issueedit is the modal that edits an issue's fields. It lists the
// fields the issue allows, edits one at a time in a form, and saves the changes
// together. When a save succeeds it closes with an Edited message.
package issueedit

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/fieldform"
	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads and saves an issue's fields.
type Backend interface {
	EditFields(iss jira.Issue) ([]jira.IssueField, error)
	Edit(key string, fields []jira.IssueField) error
	SearchUsers(issueKey, query string) ([]jira.Option, error)
}

// Edited is what the modal closes with after saving. It's a tuimsg.Finished.
type Edited struct {
	Key   string
	Count int
}

// ForParent marks Edited for the modal host.
func (Edited) ForParent() {}

// ClosesModal marks Edited for the modal host.
func (Edited) ClosesModal() {}

var _ tuimsg.Finished = Edited{}

// IssueKey implements tuimsg.Finished.
func (e Edited) IssueKey() string { return e.Key }

// Status implements tuimsg.Finished.
func (e Edited) Status() string {
	return fmt.Sprintf("Updated %d %s on %s", e.Count, fieldform.Plural(e.Count, "field"), e.Key)
}

type fieldsLoadedMsg struct {
	key    string
	fields []jira.IssueField
	err    error
}

type fieldsSavedMsg struct {
	key   string
	count int
	err   error
}

func loadFields(b Backend, iss jira.Issue) tea.Cmd {
	return func() tea.Msg {
		fs, err := b.EditFields(iss)
		return fieldsLoadedMsg{key: iss.Key, fields: fs, err: err}
	}
}

// searchUsers looks up people who can be put on the issue.
func searchUsers(b Backend, issueKey string) usersearch.Func {
	return func(query string) ([]jira.Option, error) { return b.SearchUsers(issueKey, query) }
}

// Model lists an issue's editable fields and edits one at a time in a
// form. Changes are kept until they're all saved together.
type Model struct {
	keys    fieldform.KeyMap
	key     string
	issue   jira.Issue
	backend Backend
	spin    spinner.Model
	help    help.Model

	width, height int
	// confirmClose is set while asking whether to drop unsaved changes.
	confirmClose bool

	fields fieldform.Editor
	loaded bool
	saving bool
	err    error
}

// New returns the modal that edits iss, which loads the fields when it opens.
func New(b Backend, iss jira.Issue) *Model {
	return &Model{key: iss.Key, issue: iss, backend: b, spin: pane.NewSpinner(), help: styles.Help(), keys: fieldform.NewKeyMap()}
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	return tea.Batch(loadFields(p.backend, p.issue), p.spin.Tick)
}

func (p *Model) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

func (p *Model) unsaved() int { return len(p.fields.Changed()) }

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
	case spinner.TickMsg:
		cmd = pane.Tick(&p.spin, msg, p.loading())
	case tea.KeyPressMsg:
		cmd = p.updateKey(msg)
	default:
		cmd = p.update(msg)
	}
	return p, pane.Restart(p.spin, was, p.loading(), cmd)
}

func (p *Model) updateKey(km tea.KeyPressMsg) tea.Cmd {
	if p.confirmClose {
		return p.updateConfirm(km)
	}
	if p.fields.ConsumesEsc() && key.Matches(km, p.keys.Blur) {
		return p.update(km)
	}
	closes := p.keys.Close
	if p.fields.Capturing() {
		closes = p.keys.Blur
	}
	if key.Matches(km, closes) {
		if p.unsaved() > 0 {
			p.confirmClose = true
			return nil
		}
		return tuimsg.Close()
	}
	return p.update(km)
}

func (p *Model) updateConfirm(km tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(km, p.keys.Confirm):
		return tuimsg.Close()
	case key.Matches(km, p.keys.Deny):
		p.confirmClose = false
	case key.Matches(km, p.keys.Send):
		p.confirmClose = false
		return p.update(km)
	}
	return nil
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case fieldsLoadedMsg:
		p.loaded, p.err = true, msg.err
		p.fields.Load(msg.fields, searchUsers(p.backend, p.key))
		return nil
	case fieldsSavedMsg:
		p.saving, p.err = false, msg.err
		if msg.err == nil {
			return tuimsg.Send(Edited{Key: msg.key, Count: msg.count})
		}
		return nil
	}
	if !p.loaded || p.saving || p.fields.Len() == 0 {
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && !p.fields.Holding() && key.Matches(km, p.keys.Send) {
		return p.save()
	}
	return p.fields.Update(msg)
}

func (p *Model) save() tea.Cmd {
	changed := p.fields.Changed()
	if p.saving || len(changed) == 0 {
		return nil
	}
	p.saving, p.err = true, nil
	b, issueKey := p.backend, p.key
	return func() tea.Msg {
		return fieldsSavedMsg{key: issueKey, count: len(changed), err: b.Edit(issueKey, changed)}
	}
}

// Title implements modal.Titled.
func (p *Model) Title() string { return pane.Title("Edit", p.key) }

// Footer implements modal.Footed.
func (p *Model) Footer() (string, help.KeyMap) {
	if n := p.unsaved(); n > 0 && p.confirmClose {
		return styles.Warn().Render("Discard " + fieldform.Changes(n) + "? "), p.keys.WithMode(fieldform.HelpDiscard)
	}
	return "", p.keys.WithMode(fieldform.HelpPane)
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	p.help.SetWidth(p.width)
	var warn string
	if n := p.unsaved(); n > 0 {
		warn = styles.Warn().Render("● " + fieldform.Changes(n))
		if !p.fields.Holding() {
			warn += styles.Dim().Render(" · ") + p.help.View(p.keys.WithMode(fieldform.HelpSave))
		}
	}
	spin := p.spin.View()
	lines := []string{warn}
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && p.fields.Len() == 0 && p.err == nil:
		lines = append(lines, styles.Dim().Render("No editable fields."))
	case p.loaded:
		lines = append(lines, p.fields.View(p.width, max(1, p.height-pane.BodyLines)))
	}
	lines = append(lines, "", pane.StatusLine(spin, p.width, p.err, p.saving, "Saving…", p.fields.Hint(p.help)))
	return tea.NewView(strings.Join(lines, "\n"))
}
