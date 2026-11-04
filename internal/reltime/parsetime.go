package reltime

import (
	"errors"
	"strings"
	"time"
)

// absoluteLayouts are the date and time forms ParseTime accepts besides
// relative times.
var absoluteLayouts = []string{
	"2006-01-02T15:04:05.000-0700",
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	time.DateOnly,
}

// ParseTime reads s as an absolute date or time, or else as a time relative to
// now such as 2d or -1w3d. dateOnly only changes the wording of the error.
func ParseTime(s string, now time.Time, dateOnly bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range absoluteLayouts {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t, nil
		}
	}
	t, err := Parse(s, now)
	if err != nil {
		if dateOnly {
			return t, errors.New("must be a date like 2006-01-02, or relative like -2d")
		}
		return t, errors.New("must be a date and time like 2006-01-02T15:04, or relative like -2d")
	}
	return t, nil
}
