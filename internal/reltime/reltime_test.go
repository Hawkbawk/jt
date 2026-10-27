package reltime

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, time.October, 26, 14, 30, 15, 0, time.UTC)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"0", now},
		{"+0", now},
		{"30s", now.Add(30 * time.Second)},
		{"12sec", now.Add(12 * time.Second)},
		{"45min", now.Add(45 * time.Minute)},
		{"3h", now.Add(3 * time.Hour)},
		{"30d", time.Date(2026, time.November, 25, 14, 30, 15, 0, time.UTC)},
		{"2w", time.Date(2026, time.November, 9, 14, 30, 15, 0, time.UTC)},
		{"1m", time.Date(2026, time.November, 26, 14, 30, 15, 0, time.UTC)},
		{"1y", time.Date(2027, time.October, 26, 14, 30, 15, 0, time.UTC)},
		{"+5d", time.Date(2026, time.October, 31, 14, 30, 15, 0, time.UTC)},
		{"-30d", time.Date(2026, time.September, 26, 14, 30, 15, 0, time.UTC)},
		{"-1m", time.Date(2026, time.September, 26, 14, 30, 15, 0, time.UTC)},
		{"-1y", time.Date(2025, time.October, 26, 14, 30, 15, 0, time.UTC)},
		{"14m", time.Date(2027, time.December, 26, 14, 30, 15, 0, time.UTC)},
		{"-10m", time.Date(2025, time.December, 26, 14, 30, 15, 0, time.UTC)},
		{"1.5h", now.Add(90 * time.Minute)},
		{".5h", now.Add(30 * time.Minute)},
		{"2.h", now.Add(2 * time.Hour)},
		{"1d12h", time.Date(2026, time.October, 28, 2, 30, 15, 0, time.UTC)},
		{"-1d12h", time.Date(2026, time.October, 25, 2, 30, 15, 0, time.UTC)},
		{"1m1d1h1min1s", time.Date(2026, time.November, 27, 15, 31, 16, 0, time.UTC)},
		{"1h1h", now.Add(2 * time.Hour)},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in, now)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("Parse(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseMinuteAndMonthDiffer(t *testing.T) {
	month, _ := Parse("1m", now)
	minute, _ := Parse("1min", now)
	if month.Sub(now) < 24*time.Hour || minute.Sub(now) != time.Minute {
		t.Errorf("1m = %v, 1min = %v", month, minute)
	}
}

func TestParseMonthsCrossYearBoundaries(t *testing.T) {
	tests := []struct {
		from time.Time
		in   string
		want time.Time
	}{
		{time.Date(2026, time.January, 15, 9, 0, 0, 0, time.UTC), "-1m", time.Date(2025, time.December, 15, 9, 0, 0, 0, time.UTC)},
		{time.Date(2026, time.December, 15, 9, 0, 0, 0, time.UTC), "1m", time.Date(2027, time.January, 15, 9, 0, 0, 0, time.UTC)},
		{time.Date(2028, time.February, 29, 9, 0, 0, 0, time.UTC), "-1y", time.Date(2027, time.March, 1, 9, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in, tt.from)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("Parse(%q) from %v = %v, %v; want %v", tt.in, tt.from, got, err, tt.want)
		}
	}
}

func TestParseMonthsRollOverLikeAddDate(t *testing.T) {
	from := time.Date(2026, time.January, 31, 9, 0, 0, 0, time.UTC)
	got, err := Parse("1m", from)
	if want := from.AddDate(0, 1, 0); err != nil || !got.Equal(want) {
		t.Errorf("1m = %v, %v; want %v", got, err, want)
	}
}

func TestParseDaysKeepWallClockAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	from := time.Date(2026, time.March, 1, 15, 0, 0, 0, ny)
	got, err := Parse("30d", from)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.March, 31, 15, 0, 0, 0, ny); !got.Equal(want) {
		t.Errorf("30d = %v, want %v", got, want)
	}
	if got.Sub(from) == 30*24*time.Hour {
		t.Error("30d should not be a fixed 720 hours across the spring change")
	}
	got, _ = Parse("720h", from)
	if got.Sub(from) != 720*time.Hour {
		t.Errorf("720h = %v", got)
	}
}

func TestParseKeepsLocation(t *testing.T) {
	loc := time.FixedZone("x", 3600)
	got, err := Parse("1d", now.In(loc))
	if err != nil || got.Location() != loc {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "empty value"},
		{"+", "empty value"},
		{"-", "empty value"},
		{"30", "missing unit"},
		{"d", "expected a number"},
		{"1d2", "missing unit"},
		{"30x", `unknown unit "x"`},
		{"30D", `unknown unit "D"`},
		{"1M", `unknown unit "M"`},
		{"1 d", `unknown unit " d"`},
		{"1d ", `unknown unit "d "`},
		{"1.5d", "whole number"},
		{"0.5m", "whole number"},
		{"1.2.3h", "not a number"},
		{"--1d", "expected a number"},
		{"1d-1h", `unknown unit "d-"`},
		{"10000000d", "too large"},
		{"10000000h", "too large"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in, now)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) error = %v, want one containing %q", tt.in, err, tt.want)
		}
	}
}

func TestParseErrorNamesTheInput(t *testing.T) {
	_, err := Parse("30x", now)
	if err == nil || !strings.Contains(err.Error(), `"30x"`) {
		t.Errorf("error should quote the whole input, got %v", err)
	}
}

func TestParseRejectsSumsThatOverflow(t *testing.T) {
	huge := strings.Repeat("1000000h", 400)
	if _, err := Parse(huge, now); err == nil {
		t.Error("expected an error for a sum that overflows")
	}
}

func TestCustomUnits(t *testing.T) {
	u := DefaultUnits()
	u["sprint"] = Unit{Days: 14}
	u["quarter"] = Unit{Months: 3}
	u["workday"] = Unit{Duration: 8 * time.Hour}
	u["eom"] = Unit{Months: 1, Days: -1}

	tests := []struct {
		in   string
		want time.Time
	}{
		{"2sprint", time.Date(2026, time.November, 23, 14, 30, 15, 0, time.UTC)},
		{"-1quarter", time.Date(2026, time.July, 26, 14, 30, 15, 0, time.UTC)},
		{"1.5workday", now.Add(12 * time.Hour)},
		{"1eom", time.Date(2026, time.November, 25, 14, 30, 15, 0, time.UTC)},
		{"1sprint1d", time.Date(2026, time.November, 10, 14, 30, 15, 0, time.UTC)},
		{"1d", time.Date(2026, time.October, 27, 14, 30, 15, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := u.Parse(tt.in, now)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("Parse(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}
}

func TestCustomUnitCanReplaceADefault(t *testing.T) {
	u := DefaultUnits()
	u["m"] = Unit{Duration: time.Minute}
	got, err := u.Parse("5m", now)
	if err != nil || !got.Equal(now.Add(5*time.Minute)) {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestUnitsWithoutDefaultsOnlyKnowTheirOwn(t *testing.T) {
	u := Units{"sprint": {Days: 14}}
	if _, err := u.Parse("1d", now); err == nil {
		t.Error("1d should be unknown without the defaults")
	}
	if _, err := u.Parse("1sprint", now); err != nil {
		t.Error(err)
	}
}

func TestCustomUnitWithFractionalCalendarPartIsRejected(t *testing.T) {
	u := Units{"sprint": {Days: 14}}
	if _, err := u.Parse("0.5sprint", now); err == nil {
		t.Error("expected an error")
	}
}

func TestDefaultUnitsReturnsACopy(t *testing.T) {
	u := DefaultUnits()
	u["d"] = Unit{Duration: time.Second}
	got, err := Parse("1d", now)
	if err != nil || !got.Equal(now.AddDate(0, 0, 1)) {
		t.Errorf("changing a copy changed the defaults: %v, %v", got, err)
	}
}
