// Package commandpicker is a tea.Model that lets the user choose a configured
// command and reports it to its host.
package commandpicker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/command"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads the ticket data available to command templates.
type Backend interface {
	Issue(key string) (*jira.IssueDetail, error)
	Comments(key string) (*jira.CommentList, error)
	IssueURL(key string) string
}

// Picker lists the configured commands.
type Picker struct {
	commands []config.Command
	issue    jira.Issue
	project  string
	board    string
	choice   int
	field    *huh.Select[int]
	form     *huh.Form
	keys     popup.KeyMap
	width    int
	height   int
}

// New builds a picker with commands available for issue.
func New(commands []config.Command, issue jira.Issue, project, board string) *Picker {
	k := keys.Get()
	p := &Picker{
		commands: commands, issue: issue, project: project, board: board,
		keys: popup.NewKeyMap(k.Close).WithLead(keys.Describe(k.Accept, "run")),
	}
	if len(commands) > 0 {
		opts := make([]huh.Option[int], 0, len(commands))
		for i, c := range commands {
			opts = append(opts, huh.NewOption(c.DisplayName(), i))
		}
		p.field = huh.NewSelect[int]().Title("Run a command").Options(opts...).Value(&p.choice)
		p.form = pane.NewForm(p.field)
		km := huh.NewDefaultKeyMap()
		km.Quit = key.NewBinding(key.WithDisabled())
		p.form.WithKeyMap(km)
	}
	return p
}

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd {
	if p.form == nil {
		return nil
	}
	return p.form.Init()
}

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		p.width, p.height = size.Width, size.Height
		return p, nil
	}
	if p.form == nil {
		if km, ok := msg.(tea.KeyPressMsg); ok && key.Matches(km, p.keys.Close()) {
			return p, popup.Close()
		}
		return p, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, p.keys.Close()) && !p.consumesEsc():
			return p, popup.Close()
		case key.Matches(km, keys.Get().Dismiss) && p.consumesEsc():
			_, cmd := p.form.Update(msg)
			return p, cmd
		}
	}
	if p.form.State == huh.StateCompleted {
		return p, nil
	}
	_, cmd := p.form.Update(msg)
	if p.form.State == huh.StateCompleted {
		return p, popup.CloseWith(tuimsg.CommandSelected{
			Command: p.commands[p.choice], Issue: p.issue, Project: p.project, Board: p.board,
		})
	}
	return p, cmd
}

func (p *Picker) consumesEsc() bool {
	return p.field != nil && key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, p.field.KeyBinds()...)
}

// View implements tea.Model.
func (p *Picker) View() tea.View {
	var body string
	if p.form == nil {
		body = styles.Dim().Render("No commands configured.")
	} else {
		p.form.WithWidth(p.width).WithHeight(max(1, p.height-2))
		body = p.form.View()
	}
	km := p.keys.WithForm(nil, false)
	h := styles.Help()
	h.SetWidth(max(0, p.width))
	help := h.View(km)
	return tea.NewView(popup.Stack(p.height, []string{"Commands", ""}, strings.Split(body, "\n"), help))
}

// Load fetches ticket details and comments before rendering the selected command.
func Load(b Backend, selected tuimsg.CommandSelected) tea.Cmd {
	return func() tea.Msg {
		detail, err := b.Issue(selected.Issue.Key)
		if err != nil {
			return tuimsg.CommandLoaded{Command: selected.Command, Err: err}
		}
		comments, err := b.Comments(selected.Issue.Key)
		if err != nil {
			return tuimsg.CommandLoaded{Command: selected.Command, Err: err}
		}
		title, description := selected.Issue.Summary, ""
		if detail != nil {
			if detail.Summary != "" {
				title = detail.Summary
			}
			description = detail.Description
		}
		var commentItems []jira.Comment
		if comments != nil {
			commentItems = comments.Items
		}
		return tuimsg.CommandLoaded{
			Command: selected.Command,
			Data: command.Data{
				Key: selected.Issue.Key, Project: selected.Project, Board: selected.Board,
				URL: b.IssueURL(selected.Issue.Key), Title: title,
				Description: description, Comments: commentItems,
			},
		}
	}
}

// Start renders the arguments and returns an asynchronous command that runs the executable.
func Start(loaded tuimsg.CommandLoaded) (tea.Cmd, error) {
	args, err := command.RenderArgs(loaded.Command.Args, loaded.Data)
	if err != nil {
		return nil, fmt.Errorf("preparing command: %w", err)
	}
	if strings.TrimSpace(loaded.Command.Command) == "" {
		return nil, fmt.Errorf("preparing command: command is empty")
	}
	return func() tea.Msg {
		cmd := exec.Command(loaded.Command.Command, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		return tuimsg.CommandFinished{Command: loaded.Command, Err: err, Stderr: stderr.String()}
	}, nil
}

// Handle processes shared command messages for a screen.
func Handle(b Backend, msg tea.Msg) (tea.Cmd, string, bool) {
	switch msg := msg.(type) {
	case tuimsg.CommandSelected:
		return Load(b, msg), styles.Dim().Render("Preparing " + msg.Command.DisplayName() + "…"), true
	case tuimsg.CommandLoaded:
		if msg.Err != nil {
			return nil, styles.Error().Render("preparing " + msg.Command.DisplayName() + ": " + msg.Err.Error()), true
		}
		cmd, err := Start(msg)
		if err != nil {
			return nil, styles.Error().Render(err.Error()), true
		}
		return cmd, "", true
	case tuimsg.CommandFinished:
		if msg.Err != nil {
			return nil, styles.Error().Render(msg.Status()), true
		}
		return nil, styles.Dim().Render(msg.Status()), true
	default:
		return nil, "", false
	}
}
