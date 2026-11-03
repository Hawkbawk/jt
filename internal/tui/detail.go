package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/issueview"
)

// hintLines is the height of the key hint under an issue's text.
const hintLines = 1

// markdownRenderer renders markdown wrapped to width.
type markdownRenderer func(md string, width int) string

// helpView renders a help footer in the given width.
func helpView(km help.KeyMap, width int) string {
	h := styles.Help()
	h.SetWidth(max(0, width))
	return h.View(km)
}

// detailPopup shows an issue's details.
type detailPopup struct {
	issue *issueview.Model
}

func newDetailPopup(b Backend, issueKey string, md markdownRenderer) *detailPopup {
	return &detailPopup{issue: issueview.Details(b, issueKey, md)}
}

// Init implements tea.Model.
func (p *detailPopup) Init() tea.Cmd { return p.issue.Init() }

// Update implements tea.Model.
func (p *detailPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		msg.Height -= hintLines
		_, cmd := p.issue.Update(msg)
		return p, cmd
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.close) {
			return p, popup.Close()
		}
	}
	_, cmd := p.issue.Update(msg)
	return p, cmd
}

// View implements tea.Model.
func (p *detailPopup) View() tea.View {
	var hint string
	if p.issue.Ready() {
		prefix := styles.Dim().Render(fmt.Sprintf("%3.f%% · ", p.issue.ScrollPercent()*100))
		hint = prefix + helpView(viewHelpKeys, p.issue.Width()-lipgloss.Width(prefix))
	}
	return tea.NewView(strings.Join([]string{p.issue.View().Content, hint}, "\n"))
}
