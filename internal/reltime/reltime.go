// Package reltime parses relative times such as "30d", "-2w" or "1d12h" into
// an absolute time. The syntax follows time.ParseDuration: an optional sign,
// then one or more numbers each followed by a unit. A positive value is in the
// future and a negative one is in the past.
//
// Unlike time.ParseDuration, units can be calendar based. Those are added with
// time.Time.AddDate, so a day keeps the wall-clock time across daylight saving
// changes, and a month that is too short for the day rolls over as AddDate
// does. The units are configurable, so callers can add their own.
package reltime

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
)

// Unit is the amount of time one of a unit's token stands for. The parts add
// up, so a fortnight is Unit{Days: 14} and a business quarter might be
// Unit{Months: 3, Days: -1}.
type Unit struct {
	// Months and Days are calendar amounts. Values using them must be whole
	// numbers.
	Months int
	Days   int
	// Duration is an exact amount of time. Values using it can be fractional.
	Duration time.Duration
}

// Units maps tokens to the amount of time they stand for. Tokens are case
// sensitive, and can be any text without digits or dots.
type Units map[string]Unit

// DefaultUnits returns a copy of the built-in units to add to.
//
//	s, sec          seconds
//	min             minutes
//	h               hours
//	d               calendar days
//	w               calendar weeks
//	m               calendar months
//	y               calendar years
//
// Note that m is months, as time.ParseDuration's minutes are min here.
func DefaultUnits() Units {
	return Units{
		"s":   {Duration: time.Second},
		"sec": {Duration: time.Second},
		"min": {Duration: time.Minute},
		"h":   {Duration: time.Hour},
		"d":   {Days: 1},
		"w":   {Days: 7},
		"m":   {Months: 1},
		"y":   {Months: 12},
	}
}

var defaultUnits = DefaultUnits()

// maxCount bounds how many of a unit one value can use, so that sums stay in
// range.
const maxCount = 1e6

// Parse resolves s relative to now using the default units.
func Parse(s string, now time.Time) (time.Time, error) {
	return defaultUnits.Parse(s, now)
}

// Parse resolves s relative to now. A bare "0" is allowed without a unit and
// gives now.
func (u Units) Parse(s string, now time.Time) (time.Time, error) {
	orig := s
	sign := 1
	if s != "" && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	if s == "0" {
		return now, nil
	}
	if s == "" {
		return time.Time{}, invalid(orig, errors.New("empty value"))
	}

	var months, days int
	var dur float64
	for s != "" {
		var n float64
		var name string
		var err error
		n, name, s, err = next(s)
		if err != nil {
			return time.Time{}, invalid(orig, err)
		}
		unit, ok := u[name]
		if !ok {
			return time.Time{}, invalid(orig, fmt.Errorf("unknown unit %q", name))
		}
		if n > maxCount {
			return time.Time{}, invalid(orig, errors.New("value is too large"))
		}
		if unit.Months != 0 || unit.Days != 0 {
			if n != math.Trunc(n) {
				return time.Time{}, invalid(orig, fmt.Errorf("%q needs a whole number", name))
			}
			months += int(n) * unit.Months
			days += int(n) * unit.Days
		}
		dur += n * float64(unit.Duration)
	}
	if math.Abs(dur) > math.MaxInt64/2 {
		return time.Time{}, invalid(orig, errors.New("value is too large"))
	}

	t := now.AddDate(0, sign*months, sign*days)
	return t.Add(time.Duration(float64(sign) * dur)), nil
}

// next splits the first number and unit off s.
func next(s string) (n float64, unit, rest string, err error) {
	i := 0
	for i < len(s) && (isDigit(s[i]) || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, "", "", fmt.Errorf("expected a number at %q", s)
	}
	text := s[:i]
	n, err = strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("%q is not a number", text)
	}
	s = s[i:]
	j := 0
	for j < len(s) && s[j] != '.' && !isDigit(s[j]) {
		j++
	}
	if j == 0 {
		return 0, "", "", fmt.Errorf("missing unit after %q", text)
	}
	return n, s[:j], s[j:], nil
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func invalid(s string, err error) error {
	return fmt.Errorf("reltime: invalid value %q: %w", s, err)
}
