// Package setup runs jt init: it asks how to reach Jira, checks the login,
// lets the user pick a default project and board, and saves the result.
package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

const apiTokenURL = "https://id.atlassian.com/manage-profile/security/api-tokens"

// answers collects what the user enters across the forms.
type answers struct {
	server  string
	login   string
	token   string
	project string
	boardID int
}

func fromConfig(j config.Jira) answers {
	return answers{server: j.Server, login: j.Login, project: j.Project, boardID: j.BoardID}
}

func (a answers) jira() config.Jira {
	return config.Jira{
		Server:  strings.TrimSpace(a.server),
		Login:   strings.TrimSpace(a.login),
		Project: a.project,
		BoardID: a.boardID,
	}
}

// Run walks the user through setting up the Jira connection and saves it to
// cfg and the keyring. Output that isn't part of a form goes to out.
func Run(cfg config.Config, out io.Writer) error {
	ok, err := overwriteOK(cfg.Jira, confirmOverwrite)
	if err == nil && !ok {
		_, err = fmt.Fprintf(out, "Kept your settings in %s.\n", cfg.Path())
		return err
	}
	a := fromConfig(cfg.Jira)
	if err == nil {
		err = run(cfg, &a)
	}
	if errors.Is(err, huh.ErrUserAborted) {
		_, err = fmt.Fprintln(out, "Cancelled, nothing was saved.")
		return err
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Saved your settings to %s. Run jt to open your board.\n", cfg.Path())
	return err
}

// overwriteOK reports whether init may replace the saved Jira settings,
// asking with confirm when there are some.
func overwriteOK(j config.Jira, confirm func(config.Jira) (bool, error)) (bool, error) {
	if !j.Configured() {
		return true, nil
	}
	return confirm(j)
}

func confirmOverwrite(j config.Jira) (bool, error) {
	var ok bool
	err := form(huh.NewGroup(
		huh.NewConfirm().Title("Replace your Jira settings?").
			Description(fmt.Sprintf("jt is already set up for %s as %s.", j.Server, j.Login)).
			Affirmative("Replace").Negative("Keep").Value(&ok),
	)).Run()
	return ok, err
}

func run(cfg config.Config, a *answers) error {
	if err := form(loginGroup(a)).Run(); err != nil {
		return err
	}

	client := jira.New(a.jira(), a.token)
	var projects []jira.Project
	if err := wait("Checking your login...", func() error {
		if err := client.CheckLogin(); err != nil {
			return fmt.Errorf("checking your login: %w", err)
		}
		var err error
		if projects, err = client.Projects(); err != nil {
			return fmt.Errorf("listing projects: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if len(projects) == 0 {
		return errors.New("your login can't see any projects")
	}

	previous := a.project
	if err := form(projectGroup(a, projects)).Run(); err != nil {
		return err
	}
	if a.project != previous {
		a.boardID = 0
	}

	var boards []jira.BoardSummary
	if err := wait("Loading boards...", func() error {
		var err error
		boards, err = client.Boards(a.project)
		return err
	}); err != nil {
		return fmt.Errorf("listing boards for %s: %w", a.project, err)
	}
	if len(boards) > 0 {
		if err := form(boardGroup(a, boards)).Run(); err != nil {
			return err
		}
	}

	return save(cfg, *a)
}

// save stores the token in the keyring and the rest in the settings file.
func save(cfg config.Config, a answers) error {
	j := a.jira()
	if err := config.SaveToken(j.Login, a.token); err != nil {
		return fmt.Errorf("saving the API token to the keyring: %w", err)
	}
	if err := cfg.SaveJira(j); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}

func form(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(styles.FormTheme())
}

func wait(title string, action func() error) error {
	return spinner.New().Title(title).ActionWithErr(func(context.Context) error {
		return action()
	}).Run()
}

func loginGroup(a *answers) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("Jira site").
			Placeholder("https://example.atlassian.net").
			Value(&a.server).Validate(validateServer),
		huh.NewInput().Title("Email").
			Description("The email you sign in to Jira with.").
			Value(&a.login).Validate(required),
		huh.NewInput().Title("API token").
			Description("Create one at "+apiTokenURL+". It's saved to your OS keyring.").
			EchoMode(huh.EchoModePassword).
			Value(&a.token).Validate(required),
	)
}

func projectGroup(a *answers, projects []jira.Project) *huh.Group {
	opts := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		opts[i] = huh.NewOption(p.Key+"  "+p.Name, p.Key)
	}
	return huh.NewGroup(
		huh.NewSelect[string]().Title("Default project").
			Description("jt opens this project's board on start. Press / to filter.").
			Options(opts...).Height(listHeight(len(opts)) + 1).Value(&a.project),
	)
}

func boardGroup(a *answers, boards []jira.BoardSummary) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[int]().Title("Default board").
			Options(boardOptions(boards)...).Height(listHeight(len(boards) + 1)).Value(&a.boardID),
	)
}

func boardOptions(boards []jira.BoardSummary) []huh.Option[int] {
	opts := []huh.Option[int]{huh.NewOption("Let jt pick", 0)}
	for _, b := range boards {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s (%s)", b.Name, b.Type), b.ID))
	}
	return opts
}

// listHeight keeps long lists from taking over the terminal.
func listHeight(n int) int {
	return min(n, 10) + 1
}

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("required")
	}
	return nil
}

func validateServer(s string) error {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("enter a URL like https://example.atlassian.net")
	}
	return nil
}
