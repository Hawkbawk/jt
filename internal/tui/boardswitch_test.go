package tui

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/jira"
)

func boardsBackend() *fakeBackend {
	fb := pickerBackend()
	fb.boards = []jira.BoardSummary{{ID: 1, Name: "Test Board", Type: "kanban"}, {ID: 7, Name: "Team", Type: "scrum"}}
	return fb
}

func newSettingsModel(t *testing.T, project string, fb *fakeBackend, fs *fakeSettings) BoardModel {
	t.Helper()
	fb.defaultProject = project
	if fb.board == nil {
		fb.board = testBoard()
	}
	next, _ := New(fb, fs).Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return drain(t, next.(BoardModel), next.(BoardModel).Init())
}

func TestPickingABoardOpensItWithoutSaving(t *testing.T) {
	fb, fs := boardsBackend(), &fakeSettings{}
	m := newSettingsModel(t, "TEST", fb, fs)

	m = pressDrain(t, m, "b")
	if _, ok := m.popup.Content().(*boardpicker.Picker); !ok {
		t.Fatalf("board popup not open, got %T", m.popup.Content())
	}
	m = pressDrain(t, m, "j", "enter")

	if got := fb.boardIDCalls[len(fb.boardIDCalls)-1]; got != 7 {
		t.Fatalf("loaded board %d, want 7", got)
	}
	if len(fs.boards) != 0 {
		t.Fatalf("saved boards = %v, want none", fs.boards)
	}
	if m.popup.IsOpen() || m.board.nav == nil {
		t.Fatal("the board should show once it loads")
	}
}

func TestFavoritingTheBoardSavesIt(t *testing.T) {
	fb, fs := boardsBackend(), &fakeSettings{}
	fb.board = testBoard()
	fb.board.ID = 7
	m := newSettingsModel(t, "TEST", fb, fs)

	pressDrain(t, m, "b", "j", "enter", "f")
	if !slices.Equal(fs.boards, []int{7}) {
		t.Fatalf("saved boards = %v, want [7]", fs.boards)
	}
}

func TestFavoritingABoardIgnoresOtherProjects(t *testing.T) {
	fb, fs := boardsBackend(), &fakeSettings{}
	m := newSettingsModel(t, "OTHER", fb, fs)
	m = pressDrain(t, m, "ctrl+p", "j", "enter")
	if m.board.project != "BETA" {
		t.Fatalf("project = %s", m.board.project)
	}

	m = pressDrain(t, m, "b", "j", "enter", "f")
	if len(fs.boards) != 0 {
		t.Fatalf("saved boards = %v, want none", fs.boards)
	}
	if m.status == "" {
		t.Fatal("expected a status explaining why nothing was saved")
	}
	if m.board.boardID != 7 {
		t.Fatalf("boardID = %d, want 7", m.board.boardID)
	}
}

func TestSwitchingBoardClearsFilterAndProjectKeepsItsBoard(t *testing.T) {
	fb := boardsBackend()
	m := newSettingsModel(t, "ALPHA", fb, &fakeSettings{})
	m.board.filter = &jira.Filter{Name: "Bugs", JQL: "issuetype = Bug"}

	m = pressDrain(t, m, "b", "j", "enter")
	if m.board.filter != nil {
		t.Fatal("a filter from the old board should be dropped")
	}

	m = pressDrain(t, m, "ctrl+p", "j", "enter")
	if m.board.boardID != 0 {
		t.Fatalf("a new project should start on its own board, got %d", m.board.boardID)
	}
	m = pressDrain(t, m, "ctrl+p", "enter")
	if m.board.project != "ALPHA" || m.board.boardID != 7 {
		t.Fatalf("returning to ALPHA opened board %d, want 7", m.board.boardID)
	}
}

func TestBoardSaveErrorShowsInStatus(t *testing.T) {
	m := newSettingsModel(t, "TEST", boardsBackend(), &fakeSettings{err: errors.New("read-only")})
	m = pressDrain(t, m, "b", "j", "enter", "f")
	if m.status == "" {
		t.Fatal("expected the save error in the status line")
	}
}

func TestLateBoardForAnotherBoardIsIgnored(t *testing.T) {
	m := newSettingsModel(t, "TEST", boardsBackend(), &fakeSettings{})
	m.board.boardID = 7
	nav := m.board.nav
	m.board.update(boardLoadedMsg{project: "TEST", boardID: 1, board: testBoard()})
	if m.board.nav != nav {
		t.Fatal("a result for the previous board replaced the current one")
	}
}
