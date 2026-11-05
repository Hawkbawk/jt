package board

import (
	"errors"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func boardsBackend() *tuitest.Backend {
	fb := pickerBackend()
	fb.BoardList = []jira.BoardSummary{{ID: 1, Name: "Test Board", Type: "kanban"}, {ID: 7, Name: "Team", Type: "scrum"}}
	return fb
}

func newSettingsModel(t *testing.T, project string, fb *tuitest.Backend, fs *tuitest.Settings) Model {
	t.Helper()
	fb.Project = project
	if fb.BoardData == nil {
		fb.BoardData = testBoard()
	}
	next, _ := New(fb, fs).Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return tuitest.Drain(t, next.(Model), next.(Model).Init())
}

func TestPickingABoardOpensItWithoutSaving(t *testing.T) {
	fb, fs := boardsBackend(), &tuitest.Settings{}
	m := newSettingsModel(t, "TEST", fb, fs)

	m = tuitest.PressDrain(t, m, "ctrl+b")
	if _, ok := m.Popup.Content().(*boardpicker.Picker); !ok {
		t.Fatalf("board popup not open, got %T", m.Popup.Content())
	}
	m = tuitest.PressDrain(t, m, "j", "enter")

	if got := fb.BoardIDCalls[len(fb.BoardIDCalls)-1]; got != 7 {
		t.Fatalf("loaded board %d, want 7", got)
	}
	if len(fs.Boards) != 0 {
		t.Fatalf("saved boards = %v, want none", fs.Boards)
	}
	if m.Popup.IsOpen() || m.board.nav == nil {
		t.Fatal("the board should show once it loads")
	}
}

func TestFavoritingTheBoardSavesIt(t *testing.T) {
	fb, fs := boardsBackend(), &tuitest.Settings{}
	fb.BoardData = testBoard()
	fb.BoardData.ID = 7
	m := newSettingsModel(t, "TEST", fb, fs)

	tuitest.PressDrain(t, m, "ctrl+b", "j", "enter", "ctrl+b", "f")
	if !slices.Equal(fs.Boards, []int{7}) {
		t.Fatalf("saved boards = %v, want [7]", fs.Boards)
	}
}

func TestFavoritingABoardIgnoresOtherProjects(t *testing.T) {
	fb, fs := boardsBackend(), &tuitest.Settings{}
	m := newSettingsModel(t, "OTHER", fb, fs)
	m = tuitest.PressDrain(t, m, "ctrl+p", "j", "enter")
	if m.board.project != "BETA" {
		t.Fatalf("project = %s", m.board.project)
	}

	m = tuitest.PressDrain(t, m, "ctrl+b", "j", "enter", "ctrl+b", "f")
	if len(fs.Boards) != 0 {
		t.Fatalf("saved boards = %v, want none", fs.Boards)
	}
	if m.Status == "" {
		t.Fatal("expected a status explaining why nothing was saved")
	}
	if m.board.boardID != 7 {
		t.Fatalf("boardID = %d, want 7", m.board.boardID)
	}
}

func TestSwitchingBoardClearsFilterAndProjectKeepsItsBoard(t *testing.T) {
	fb := boardsBackend()
	m := newSettingsModel(t, "ALPHA", fb, &tuitest.Settings{})
	m.board.filter = &jira.Filter{Name: "Bugs", JQL: "issuetype = Bug"}

	m = tuitest.PressDrain(t, m, "ctrl+b", "j", "enter")
	if m.board.filter != nil {
		t.Fatal("a filter from the old board should be dropped")
	}

	m = tuitest.PressDrain(t, m, "ctrl+p", "j", "enter")
	if m.board.boardID != 0 {
		t.Fatalf("a new project should start on its own board, got %d", m.board.boardID)
	}
	m = tuitest.PressDrain(t, m, "ctrl+p", "enter")
	if m.board.project != "ALPHA" || m.board.boardID != 7 {
		t.Fatalf("returning to ALPHA opened board %d, want 7", m.board.boardID)
	}
}

func TestBoardSaveErrorShowsInStatus(t *testing.T) {
	m := newSettingsModel(t, "TEST", boardsBackend(), &tuitest.Settings{Err: errors.New("read-only")})
	m = tuitest.PressDrain(t, m, "ctrl+b", "j", "enter", "ctrl+b", "f")
	if m.Status == "" {
		t.Fatal("expected the save error in the status line")
	}
}

func TestLateBoardForAnotherBoardIsIgnored(t *testing.T) {
	m := newSettingsModel(t, "TEST", boardsBackend(), &tuitest.Settings{})
	m.board.boardID = 7
	nav := m.board.nav
	m.board.update(boardLoadedMsg{project: "TEST", boardID: 1, board: testBoard()})
	if m.board.nav != nav {
		t.Fatal("a result for the previous board replaced the current one")
	}
}

func TestFavoritingAProjectSavesItAndKeepsThePickerOpen(t *testing.T) {
	fb, fs := boardsBackend(), &tuitest.Settings{}
	m := newSettingsModel(t, "ALPHA", fb, fs)

	m = tuitest.PressDrain(t, m, "ctrl+p", "j", "f")
	if !slices.Equal(fs.Projects, []string{"BETA"}) {
		t.Fatalf("saved projects = %v, want [BETA]", fs.Projects)
	}
	if fb.Project != "BETA" {
		t.Fatalf("default project = %q, want BETA", fb.Project)
	}
	projects(t, m)
}

func TestFavoriteProjectSaveErrorLeavesDefaultAlone(t *testing.T) {
	fb := boardsBackend()
	m := newSettingsModel(t, "ALPHA", fb, &tuitest.Settings{Err: errors.New("read-only")})

	m = tuitest.PressDrain(t, m, "ctrl+p", "j", "f")
	if fb.Project != "ALPHA" || m.Status == "" {
		t.Fatalf("default = %q, status = %q", fb.Project, m.Status)
	}
}
