package fakejira_test

import (
	"testing"

	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/tui"
)

var _ tui.Backend = (*fakejira.Backend)(nil)

func TestBackendSmoke(t *testing.T) {
	b := fakejira.New()
	if _, err := b.Projects(); err != nil {
		t.Fatal(err)
	}

	board, err := b.Board(fakejira.DefaultProject, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Columns) == 0 {
		t.Fatal("default project should have a board")
	}

	for _, column := range board.Columns {
		if len(column.Issues) == 0 {
			continue
		}
		if _, err := b.Issue(column.Issues[0].Key); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("default board should contain an issue")
}
