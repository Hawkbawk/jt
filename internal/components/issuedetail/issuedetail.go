// Package issuedetail is a tea.Model that shows an issue's details above a key
// hint. It knows nothing about where it's shown: it asks to be dismissed with
// popup.Close.
package issuedetail

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/issueview"
)

// hintLines is the height of the key hint under an issue's text.
const hintLines = 1

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
	case tea.WindowSizeMsg:
		msg.Height -= hintLines
		_, cmd := p.Issue.Update(msg)
		return p, cmd
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Get().Close) {
			return p, popup.Close()
		}
	}
	_, cmd := p.Issue.Update(msg)
	return p, cmd
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	var hint string
	if p.Issue.Ready() {
		prefix := styles.Dim().Render(fmt.Sprintf("%3.f%% · ", p.Issue.ScrollPercent()*100))
		hint = prefix + helpView(newViewKeys(), p.Issue.Width()-lipgloss.Width(prefix))
	}
	return tea.NewView(strings.Join([]string{p.Issue.View().Content, hint}, "\n"))
}

// helpView renders a help footer in the given width.
func helpView(km help.KeyMap, width int) string {
	h := styles.Help()
	h.SetWidth(max(0, width))
	return h.View(km)
}

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
