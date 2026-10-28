package styles

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	tint "github.com/lrstanley/bubbletint/v2"
)

// useRegistry gives the test its own default registry, starting on the first
// tint.
func useRegistry(t *testing.T, tints ...*tint.Tint) {
	t.Helper()
	prev := tint.DefaultRegistry
	t.Cleanup(func() { tint.DefaultRegistry = prev })
	tint.DefaultRegistry = tint.NewRegistry(tints[0], tints...)
}

// useTerminal makes src the terminal jt detects, or no terminal when it's nil.
func useTerminal(t *testing.T, src tint.Source) {
	t.Helper()
	prev := detect
	t.Cleanup(func() { detect = prev })
	detect = func() (tint.Source, error) {
		if src == nil {
			return nil, tint.ErrNotFound
		}
		return src, nil
	}
}

// fakeTerminal is a terminal source whose active tint the test controls.
type fakeTerminal struct {
	mu      sync.Mutex
	active  *tint.Tint
	err     error
	changes chan struct{}
}

func newFakeTerminal(base *tint.Tint) *fakeTerminal {
	return &fakeTerminal{active: terminalTint(base), changes: make(chan struct{}, 1)}
}

// terminalTint is base as a terminal's config would report it.
func terminalTint(base *tint.Tint) *tint.Tint {
	t := *base
	t.ID, t.DisplayName = "current", "Fake terminal"
	return &t
}

func (f *fakeTerminal) Name() string { return "fake" }

func (f *fakeTerminal) Load(context.Context) ([]*tint.Tint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return []*tint.Tint{f.active}, nil
}

func (f *fakeTerminal) Watch(ctx context.Context) (<-chan struct{}, error) {
	out := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case <-f.changes:
				out <- struct{}{}
			}
		}
	}()
	return out, nil
}

func (f *fakeTerminal) set(t *tint.Tint) {
	f.mu.Lock()
	f.active = t
	f.mu.Unlock()
	f.changes <- struct{}{}
}

func TestStylesFollowCurrentTint(t *testing.T) {
	useRegistry(t, tint.TintDraculaPlus, tint.TintGruvboxLightHard)

	if Title().GetForeground() != tint.TintDraculaPlus.Blue || Background() != tint.TintDraculaPlus.Bg {
		t.Fatal("styles should be drawn from the registry's current tint")
	}
	tint.SetTint(tint.TintGruvboxLightHard)
	if Current() != tint.TintGruvboxLightHard || Title().GetForeground() != tint.TintGruvboxLightHard.Blue {
		t.Error("styles should be rebuilt when the current tint changes")
	}
	if Highlight() == tint.TintGruvboxLightHard.Yellow {
		t.Error("a light tint's yellow should be darkened to stay readable")
	}
}

func TestStart(t *testing.T) {
	notFound := fmt.Errorf("no config: %w", tint.ErrNotFound)
	tests := []struct {
		name              string
		id                string
		loadErr           error
		noTerm            bool
		want, wantDefault string
		wantErr           bool
	}{
		{name: "follows the terminal", want: "fake:current", wantDefault: "fake:current"},
		{name: "configured tint wins", id: "nord", want: "nord", wantDefault: "fake:current"},
		{name: "unknown tint falls back to the terminal", id: "gruvbox-dark", want: "fake:current", wantDefault: "fake:current", wantErr: true},
		{name: "terminal without a config is ignored", loadErr: notFound, want: "dracula_plus", wantDefault: "dracula_plus"},
		{name: "unrecognized terminal", noTerm: true, id: "nord", want: "nord", wantDefault: "dracula_plus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useRegistry(t, tint.TintDraculaPlus, tint.TintNord)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			var src tint.Source
			if !tt.noTerm {
				f := newFakeTerminal(tint.TintCatppuccinMocha)
				f.err = tt.loadErr
				src = f
			}
			useTerminal(t, src)
			err := Start(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got := Current().ID; got != tt.want {
				t.Errorf("current = %q, want %q", got, tt.want)
			}
			if got := Default(ctx).ID; got != tt.wantDefault {
				t.Errorf("default = %q, want %q", got, tt.wantDefault)
			}
		})
	}
}

func TestTerminalEditsRestyle(t *testing.T) {
	useRegistry(t, tint.TintDraculaPlus)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	term := newFakeTerminal(tint.TintCatppuccinMocha)
	useTerminal(t, term)
	if err := Start(ctx, ""); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	t.Cleanup(tint.Subscribe(func(ev tint.Event) {
		if ev.Kind == tint.CurrentChanged {
			changed <- struct{}{}
		}
	}))

	edited := terminalTint(tint.TintNord)
	term.set(edited)
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("editing the terminal's theme should change the current tint")
	}
	if Background() != edited.Bg {
		t.Error("styles should be rebuilt from the edited terminal theme")
	}
}

func TestDefaultFollowsTerminalThemeSwitch(t *testing.T) {
	useRegistry(t, tint.TintDraculaPlus, tint.TintNord)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	term := newFakeTerminal(tint.TintCatppuccinMocha)
	useTerminal(t, term)
	if err := Start(ctx, "nord"); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	t.Cleanup(tint.Subscribe(func(ev tint.Event) {
		if ev.Kind == tint.CurrentChanged {
			changed <- struct{}{}
		}
	}))

	switched := terminalTint(tint.TintGruvboxLightHard)
	switched.ID = "other"
	term.set(switched)
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("switching the terminal's theme should change the current tint")
	}
	if got := Default(ctx).ID; got != "fake:other" {
		t.Errorf("default = %q, want the terminal's new theme", got)
	}
}

func TestStylesAreSafeConcurrently(t *testing.T) {
	useRegistry(t, tint.TintDraculaPlus, tint.TintNord)

	tints := []*tint.Tint{tint.TintDraculaPlus, tint.TintNord}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			tint.SetTint(tints[i%len(tints)])
		}()
		go func() {
			defer wg.Done()
			_ = Card().GetHorizontalFrameSize() + Popup().GetVerticalFrameSize()
		}()
	}
	wg.Wait()
}
