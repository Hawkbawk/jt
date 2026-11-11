package popup

import "testing"

func TestFitHeight(t *testing.T) {
	for _, tc := range []struct {
		in     string
		height int
		want   string
	}{
		{"a\nb\nc", 2, "a\nb"},
		{"a\nb", 2, "a\nb"},
		{"a", 3, "a\n\n"},
		{"a\n\nb\n", 3, "a\n\nb"},
		{"a", 0, ""},
	} {
		if got := fitHeight(tc.in, tc.height); got != tc.want {
			t.Errorf("fitHeight(%q, %d) = %q, want %q", tc.in, tc.height, got, tc.want)
		}
	}
}
