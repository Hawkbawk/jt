package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/styles"
)

// Settings persists the user's choices.
type Settings interface {
	SaveTheme(config.Theme) error
}

// themeSelectedMsg reports the theme picked in the theme popup.
type themeSelectedMsg struct {
	theme config.Theme
}

// themeSavedMsg reports whether the picked theme was written to the settings.
type themeSavedMsg struct {
	err error
}

func saveTheme(s Settings, t config.Theme) tea.Cmd {
	return func() tea.Msg {
		return themeSavedMsg{err: s.SaveTheme(t)}
	}
}

// themePopup lists the themes and applies the chosen one.
type themePopup struct {
	choiceList
	themes        []config.Theme
	current       config.Theme
	width, height int
}

func newThemePopup() *themePopup {
	p := &themePopup{themes: config.Themes(), current: styles.Current()}
	for i, t := range p.themes {
		if t == p.current {
			p.cursor = i
		}
	}
	return p
}

func (p *themePopup) setSize(width, height int) {
	p.width, p.height = width, height
}

func (p *themePopup) update(msg tea.Msg) (popup, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(km, keys.close):
		return nil, nil
	case p.handle(km, len(p.themes)):
	case key.Matches(km, keys.accept):
		chosen := p.themes[p.cursor]
		return nil, func() tea.Msg { return themeSelectedMsg{theme: chosen} }
	}
	return p, nil
}

func (p *themePopup) view(string) string {
	innerW, _ := popupInner(p.width, p.height)
	labels := make([]string, len(p.themes))
	for i, t := range p.themes {
		labels[i] = t.String()
		if t == p.current {
			labels[i] += " ✓"
		}
	}
	header := []string{styles.Title().Render("Theme"), ""}
	return renderPopup(p.width, p.height, header, p.render(labels, innerW), styles.Dim().Render("j/k select · enter apply · esc cancel"))
}
