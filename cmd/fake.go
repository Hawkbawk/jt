//go:build dev

package cmd

import (
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/tui"
)

func init() {
	newFakeBackend = func() tui.Backend { return fakejira.New() }
}
