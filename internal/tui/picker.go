package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ryanhawkins/jt/internal/jira"
)

type projectsLoadedMsg struct {
	projects []jira.Project
	err      error
}

type projectItem struct {
	jira.Project
	isDefault bool
}

func (i projectItem) Title() string {
	if i.isDefault {
		return i.Key + " ★"
	}
	return i.Key
}

func (i projectItem) Description() string { return i.Name }
func (i projectItem) FilterValue() string { return i.Key + " " + i.Name }

var (
	pickerOpenKey    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	pickerDefaultKey = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "set default"))
	pickerCloseKey   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))
)

// picker is the project selection popup.
type picker struct {
	list        list.Model
	loaded      bool
	err         error
	cancellable bool
}

func newPicker(cancellable bool) picker {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(highlight).BorderForeground(highlight)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(highlight).BorderForeground(highlight)

	l := list.New(nil, d, 0, 0)
	l.Title = "Select a project"
	l.Styles.Title = titleStyle
	// The list re-enables its own bindings, so quitting is removed by giving
	// those bindings no keys; jt handles quitting itself.
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		keys := []key.Binding{pickerOpenKey, pickerDefaultKey}
		if cancellable {
			keys = append(keys, pickerCloseKey)
		}
		return keys
	}
	return picker{list: l, cancellable: cancellable}
}

func (p *picker) setProjects(projects []jira.Project, defaultKey string) {
	items := make([]list.Item, len(projects))
	for i, pr := range projects {
		items[i] = projectItem{Project: pr, isDefault: pr.Key == defaultKey}
	}
	p.list.SetItems(items)
	p.loaded = true
}

// markDefault updates which item shows the default marker.
func (p *picker) markDefault(defaultKey string) {
	for i, it := range p.list.Items() {
		pi := it.(projectItem)
		pi.isDefault = pi.Key == defaultKey
		p.list.SetItem(i, pi)
	}
}

func (p *picker) selected() (jira.Project, bool) {
	it, ok := p.list.SelectedItem().(projectItem)
	return it.Project, ok
}

func (p *picker) filtering() bool {
	return p.list.FilterState() == list.Filtering
}

func (p *picker) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

func (p *picker) setSize(width, height int) {
	p.list.SetSize(width-popupStyle.GetHorizontalFrameSize(), height-popupStyle.GetVerticalFrameSize())
}

func (p *picker) view(width, height int, spin string) string {
	innerW := width - popupStyle.GetHorizontalFrameSize()
	innerH := height - popupStyle.GetVerticalFrameSize()

	var body string
	switch {
	case p.err != nil:
		body = errorStyle.Render("Error loading projects: " + p.err.Error())
	case !p.loaded:
		body = spin + " Loading projects…"
	default:
		body = p.list.View()
	}
	return popupStyle.Width(innerW + popupStyle.GetHorizontalPadding()).
		Height(innerH).
		Render(lipgloss.NewStyle().MaxWidth(innerW).Render(body))
}
