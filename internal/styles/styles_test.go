package styles

import (
	"sync"
	"testing"

	"github.com/hawkbawk/jt/internal/config"
)

func TestUseSwapsWholeSet(t *testing.T) {
	t.Cleanup(func() { use(config.ThemeTerminal, true) })

	for _, th := range config.Themes() {
		for _, dark := range []bool{true, false} {
			use(th, dark)
			if Current() != th {
				t.Errorf("Current() = %v, want %v", Current(), th)
			}
			if FormTheme() == nil || Markdown("# hi", 20) == "" {
				t.Errorf("%v: missing renderers", th)
			}
			if get().def.form(dark) == nil {
				t.Errorf("%v: nil form styles", th)
			}
		}
	}
}

func TestUnknownThemeFallsBackToBase(t *testing.T) {
	t.Cleanup(func() { use(config.ThemeTerminal, true) })

	use(config.Theme(99), true)
	want := build(config.ThemeTerminal, true)
	if Title().GetForeground() != want.title.GetForeground() {
		t.Error("unknown theme did not fall back to terminal")
	}
}

func TestUseIsSafeConcurrently(t *testing.T) {
	t.Cleanup(func() { use(config.ThemeTerminal, true) })

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			use(config.Theme(i%len(config.Themes())), true)
		}()
		go func() {
			defer wg.Done()
			_ = Card().GetHorizontalFrameSize() + Popup().GetVerticalFrameSize()
		}()
	}
	wg.Wait()
}
