package tui

import (
	"errors"
	"strings"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/reltime"
)

// jiraDateTime is the layout Jira reads and writes date time fields in.
const jiraDateTime = "2006-01-02T15:04:05.000-0700"

// absoluteLayouts are the date and time forms accepted besides relative times.
var absoluteLayouts = []string{
	jiraDateTime,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	time.DateOnly,
}

func (v *fieldValue) isTime() bool {
	return v.Kind == jira.FieldDate || v.Kind == jira.FieldDateTime
}

// parseTime reads s as an absolute date or time, or else as a time relative to
// now such as 2d or -1w3d.
func (v *fieldValue) parseTime(s string, now time.Time) (time.Time, error) {
	return parseTimeText(s, now, v.Kind == jira.FieldDate)
}

// parseTimeText reads s as an absolute date or time, or else as a time
// relative to now. dateOnly only changes the wording of the error.
func parseTimeText(s string, now time.Time, dateOnly bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range absoluteLayouts {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t, nil
		}
	}
	t, err := reltime.Parse(s, now)
	if err != nil {
		if dateOnly {
			return t, errors.New("must be a date like 2006-01-02, or relative like -2d")
		}
		return t, errors.New("must be a date and time like 2006-01-02T15:04, or relative like -2d")
	}
	return t, nil
}

// normalizeTime replaces the text entered with the time it stands for, in the
// form Jira uses. Empty text and text that doesn't parse are left alone.
func (v *fieldValue) normalizeTime(now time.Time) {
	if !v.isTime() || strings.TrimSpace(v.text) == "" {
		return
	}
	t, err := v.parseTime(v.text, now)
	if err != nil {
		return
	}
	layout := jiraDateTime
	if v.Kind == jira.FieldDate {
		layout = time.DateOnly
	}
	v.text = t.Format(layout)
}
