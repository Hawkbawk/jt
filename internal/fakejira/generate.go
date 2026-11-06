package fakejira

import (
	"fmt"
	"hash/crc32"
	"slices"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/hawkbawk/jt/internal/jira"
)

// The seed types below are filled by gofakeit from their fake tags. Anything
// that has to refer to other generated data (assignees, comment authors)
// is skipped there and picked from the pools afterwards.

// issueSeed is the generated part of an issue.
type issueSeed struct {
	Type        string        `fake:"{weighted:[Story,Task,Bug],[5,4,3]}"`
	Status      string        `fake:"{weighted:[To Do,In Progress,In Review,Done],[4,3,1,3]}"`
	Summary     string        `fake:"{randomstring:[Fix,Add,Update,Remove,Improve,Document,Investigate,Refactor,Simplify,Speed up]} {adjective} {noun}"`
	Description []string      `fake:"{loremipsumparagraph:1,4,12}" fakesize:"0,12"`
	Priority    string        `fake:"{weighted:[Highest,High,Medium,Low,Lowest],[1,3,6,3,1]}"`
	Labels      []jira.Option `fake:"{jiralabel}" fakesize:"0,3"`
	Components  []jira.Option `fake:"{jiracomponent}" fakesize:"0,2"`
	Team        jira.Option   `fake:"{jirateam}"`
	DesignDoc   string        `fake:"{url}"`
	Points      int           `fake:"{number:1,13}"`
	DueInDays   int           `fake:"{number:-10,45}"`
	AgeHours    int           `fake:"{number:2,1200}"`
	// Idle is how far through its life, as a percentage, the issue was last
	// touched.
	Idle int `fake:"{number:0,100}"`

	Assigned  bool `fake:"{chance:80}"`
	HasTeam   bool `fake:"{chance:40}"`
	HasPoints bool `fake:"{chance:50}"`
	HasDue    bool `fake:"{chance:25}"`
	HasDoc    bool `fake:"{chance:15}"`
	Reviewers int  `fake:"{number:0,2}"`

	Comments []commentSeed `fakesize:"0,150"`
}

type commentSeed struct {
	Body   string `fake:"{paragraph}"`
	Edited bool   `fake:"{chance:10}"`
	// Posted is how far through the issue's life, as a percentage, the
	// comment was written.
	Posted int `fake:"{number:1,100}"`
}

// Size is how much data the fake generates.
type Size struct {
	Users int
	// DemoIssues and OpsIssues are the issue counts for the two projects.
	DemoIssues, OpsIssues int
}

// DefaultSize is the size used unless WithSize says otherwise.
var DefaultSize = Size{Users: 28, DemoIssues: 75, OpsIssues: 30}

// normalize raises counts that are too small to build a board from.
func (s Size) normalize() Size {
	s.Users = max(s.Users, 1)
	s.DemoIssues = max(s.DemoIssues, 0)
	s.OpsIssues = max(s.OpsIssues, 0)
	return s
}

var labelNames = []jira.Option{
	{ID: "backend", Name: "backend"},
	{ID: "frontend", Name: "frontend"},
	{ID: "performance", Name: "performance"},
	{ID: "security", Name: "security"},
	{ID: "tech-debt", Name: "tech-debt"},
	{ID: "docs", Name: "docs"},
	{ID: "accessibility", Name: "accessibility"},
}

func init() {
	option := func(name, description string, generate func(*gofakeit.Faker) jira.Option) {
		gofakeit.AddFuncLookup(name, gofakeit.Info{
			Display:     name,
			Category:    "jira",
			Description: description,
			Output:      "jira.Option",
			Generate: func(f *gofakeit.Faker, _ *gofakeit.MapParams, _ *gofakeit.Info) (any, error) {
				return generate(f), nil
			},
		})
	}
	option("jirauser", "A person who can be assigned issues", func(f *gofakeit.Faker) jira.Option {
		return jira.Option{ID: "user-" + f.UUID(), Name: f.Name()}
	})
	option("jiralabel", "An issue label", func(f *gofakeit.Faker) jira.Option {
		return labelNames[f.IntN(len(labelNames))]
	})
	option("jiracomponent", "An issue component", func(f *gofakeit.Faker) jira.Option {
		return components[f.IntN(len(components))]
	})
	option("jirateam", "A team", func(f *gofakeit.Faker) jira.Option {
		return teams[f.IntN(len(teams))]
	})

	gofakeit.AddFuncLookup("chance", gofakeit.Info{
		Display:     "chance",
		Category:    "jira",
		Description: "True this often, as a percentage",
		Output:      "bool",
		Params:      []gofakeit.Param{{Field: "percent", Type: "int"}},
		Generate: func(f *gofakeit.Faker, m *gofakeit.MapParams, info *gofakeit.Info) (any, error) {
			percent, err := info.GetInt(m, "percent")
			return f.IntN(100) < percent, err
		},
	})
}

// generate fills in size's worth of users and issues.
func generate(f *gofakeit.Faker, now time.Time, size Size) ([]user, []*issue, error) {
	size = size.normalize()
	fill := func(v any) error {
		if err := f.Struct(v); err != nil {
			return fmt.Errorf("generating fake data: %w", err)
		}
		return nil
	}

	users := make([]user, size.Users)
	for i := range users {
		if err := fill(&users[i]); err != nil {
			return nil, nil, err
		}
	}

	g := &issueGen{f: f, now: now, users: users}
	var issues []*issue
	seed := func(key string, n int) error {
		for i := range n {
			var s issueSeed
			if err := fill(&s); err != nil {
				return err
			}
			issues = append(issues, g.issue(fmt.Sprintf("%s-%d", key, i+1), s))
		}
		return nil
	}
	if err := seed("DEMO", size.DemoIssues); err != nil {
		return nil, nil, err
	}
	if err := seed("OPS", size.OpsIssues); err != nil {
		return nil, nil, err
	}
	return users, issues, nil
}

type issueGen struct {
	f     *gofakeit.Faker
	now   time.Time
	users []user
}

// person picks anyone, including people who have left.
func (g *issueGen) person() jira.Option {
	return g.users[g.f.IntN(len(g.users))].Option
}

// assignable picks someone who can be assigned issues, or anyone if nobody
// is active.
func (g *issueGen) assignable() jira.Option {
	if !slices.ContainsFunc(g.users, func(u user) bool { return u.Active }) {
		return g.person()
	}
	for {
		if u := g.users[g.f.IntN(len(g.users))]; u.Active {
			return u.Option
		}
	}
}

func (g *issueGen) issue(key string, s issueSeed) *issue {
	age := time.Duration(s.AgeHours) * time.Hour
	created := g.now.Add(-age)
	iss := &issue{
		Key: key, Type: s.Type, Status: named(allStatuses, s.Status), Reporter: g.person(),
		Created: created, Updated: created.Add(age * time.Duration(s.Idle) / 100),
		Values: map[string]value{
			"summary":  {Text: s.Summary},
			"priority": {Chosen: []jira.Option{named(priorities, s.Priority)}},
			"labels":   {Chosen: uniq(s.Labels)},
		},
	}
	set := func(id string, v value) { iss.Values[id] = v }
	if s.Assigned {
		iss.Assignee = g.assignable()
	}
	if len(s.Description) > 0 {
		text := strings.Join(s.Description, "\n\n")
		switch crc32.ChecksumIEEE([]byte(key)) % 6 {
		case 0:
			text += "\n\n" + imageLink("screenshot.png", pngAttachment)
		case 1:
			text += "\n\n" + imageLink("repro.gif", gifAttachment)
		}
		set("description", value{Text: text})
	}
	if c := uniq(s.Components); len(c) > 0 {
		set("components", value{Chosen: c})
	}
	if s.HasTeam {
		set("customfield_10030", value{Chosen: []jira.Option{s.Team}})
	}
	if s.HasPoints {
		set("customfield_10016", value{Text: fmt.Sprint(s.Points)})
	}
	if s.HasDue {
		set("duedate", value{Text: g.now.AddDate(0, 0, s.DueInDays).Format(dateLayout)})
	}
	if s.HasDoc {
		set("customfield_10040", value{Text: s.DesignDoc})
	}
	var reviewers []jira.Option
	for range s.Reviewers {
		reviewers = append(reviewers, g.assignable())
	}
	if reviewers = uniq(reviewers); len(reviewers) > 0 {
		set("customfield_10020", value{Chosen: reviewers})
	}

	for _, c := range s.Comments {
		iss.Comments = append(iss.Comments, comment{
			Author: g.person(), Body: c.Body, Edited: c.Edited,
			Created: created.Add(age * time.Duration(c.Posted) / 100),
		})
	}
	slices.SortStableFunc(iss.Comments, func(a, b comment) int { return a.Created.Compare(b.Created) })
	if n := len(iss.Comments); n > 0 && iss.Comments[n-1].Created.After(iss.Updated) {
		iss.Updated = iss.Comments[n-1].Created
	}
	return iss
}

// uniq drops repeated options, keeping the first of each.
func uniq(opts []jira.Option) []jira.Option {
	var out []jira.Option
	for _, o := range opts {
		if jira.IndexID(out, o.ID) < 0 {
			out = append(out, o)
		}
	}
	return out
}

// named finds the option called name.
func named(opts []jira.Option, name string) jira.Option {
	return opts[slices.IndexFunc(opts, func(o jira.Option) bool { return o.Name == name })]
}
