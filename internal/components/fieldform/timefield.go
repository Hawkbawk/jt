package fieldform

import (
	"strings"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/reltime"
)

// jiraDateTime is the layout Jira reads and writes date time fields in.
const jiraDateTime = "2006-01-02T15:04:05.000-0700"

func (v *fieldValue) isTime() bool {
	return v.Kind == jira.FieldDate || v.Kind == jira.FieldDateTime
}

// parseTime reads s as an absolute date or time, or else as a time relative to
// now such as 2d or -1w3d.
func (v *fieldValue) parseTime(s string, now time.Time) (time.Time, error) {
	return reltime.ParseTime(s, now, v.Kind == jira.FieldDate)
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
