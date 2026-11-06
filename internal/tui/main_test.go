package tui

import (
	"os"
	"testing"

	"github.com/hawkbawk/jt/internal/components/usersearch"
)

func TestMain(m *testing.M) {
	usersearch.Delay = 0
	os.Exit(m.Run())
}
