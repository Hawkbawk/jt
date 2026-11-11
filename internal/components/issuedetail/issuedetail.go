// Package issuedetail is the popup that shows an issue's details.
package issuedetail

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/issueview"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Model shows an issue's details.
type Model struct {
	Issue *issueview.Model
}

// New returns a model that loads issueKey and renders its description with md.
func New(b issueview.Backend, issueKey string, md issueview.Markdown) *Model {
	return &Model{Issue: issueview.Details(b, issueKey, md)}
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd { return p.Issue.Init() }

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Get().Close) {
			return p, tuimsg.Close()
		}
	}
	_, cmd := p.Issue.Update(msg)
	return p, cmd
}

// Title implements popup.Titled.
func (p *Model) Title() string { return p.Issue.Title() }

// Footer implements popup.Footed.
func (p *Model) Footer() (string, help.KeyMap) {
	if !p.Issue.Ready() {
		return "", nil
	}
	return styles.Dim().Render(fmt.Sprintf("%3.f%% · ", p.Issue.ScrollPercent()*100)), newViewKeys()
}

// View implements tea.Model.
func (p *Model) View() tea.View { return p.Issue.View() }

// viewKeyMap is the help footer under an issue's details.
type viewKeyMap struct {
	scroll, topBottom, close key.Binding
}

func newViewKeys() viewKeyMap {
	k := keys.Get()
	return viewKeyMap{scroll: k.Scroll, topBottom: k.TopBottom, close: k.Close}
}

// ShortHelp implements help.KeyMap.
func (k viewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.scroll, k.topBottom, k.close}
}

// FullHelp implements help.KeyMap.
func (k viewKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
