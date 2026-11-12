package fieldform

import (
	"testing"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestFieldValueValidate(t *testing.T) {
	tests := []struct {
		kind     jira.FieldKind
		required bool
		text     string
		ok       bool
	}{
		{jira.FieldNumber, false, "3.5", true},
		{jira.FieldNumber, false, " 8 ", true},
		{jira.FieldNumber, false, "abc", false},
		{jira.FieldNumber, false, "", true},
		{jira.FieldNumber, true, "", false},
		{jira.FieldDate, false, "2026-10-26", true},
		{jira.FieldDate, false, "10/26/2026", false},
		{jira.FieldDate, false, "-2d", true},
		{jira.FieldDateTime, false, "2026-10-26T09:30", true},
		{jira.FieldDateTime, false, "1d12h", true},
		{jira.FieldDateTime, false, "soon", false},
		{jira.FieldDate, false, "2026-02-30", false},
		{jira.FieldDate, false, "", true},
	}
	for _, tt := range tests {
		v := &fieldValue{Kind: tt.kind, Required: tt.required}
		if err := v.validate(tt.text); (err == nil) != tt.ok {
			t.Errorf("validate(%v, required=%v, %q) = %v", tt.kind, tt.required, tt.text, err)
		}
	}
}

func TestNormalizeTime(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		kind jira.FieldKind
		in   string
		want string
	}{
		{jira.FieldDate, "-2d", "2026-10-25"},
		{jira.FieldDate, "2026-11-01", "2026-11-01"},
		{jira.FieldDateTime, "1d12h", "2026-10-28T21:00:00.000+0000"},
		{jira.FieldDateTime, "2026-10-26T09:30", "2026-10-26T09:30:00.000+0000"},
		{jira.FieldDateTime, "", ""},
	}
	for _, tt := range tests {
		v := &fieldValue{Kind: tt.kind, text: tt.in}
		v.normalizeTime(now)
		if v.text != tt.want {
			t.Errorf("normalizeTime(%v, %q) = %q, want %q", tt.kind, tt.in, v.text, tt.want)
		}
	}
}
