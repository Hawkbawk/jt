package cmd

import (
	"testing"

	"github.com/hawkbawk/jt/internal/fakejira"
)

func TestOpenBackendUsesFakeSeed(t *testing.T) {
	wasFake, oldVersion := fake, version
	fake, version = true, ""
	t.Cleanup(func() {
		fake, version = wasFake, oldVersion
	})
	t.Setenv("JT_FAKE_SEED", "42")

	got, _, err := openBackend()
	if err != nil {
		t.Fatal(err)
	}
	want := fakejira.NewWithSeed(42)
	gotIssues, err := got.Search(`project = "DEMO" ORDER BY key ASC`)
	if err != nil {
		t.Fatal(err)
	}
	wantIssues, err := want.Search(`project = "DEMO" ORDER BY key ASC`)
	if err != nil {
		t.Fatal(err)
	}
	if gotIssues[2].Summary != wantIssues[2].Summary {
		t.Fatal("JT_FAKE_SEED was not applied to the fake backend")
	}
}
