package tghandlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var monthNames = map[string]time.Month{
	"january": time.January, "jan": time.January,
	"february": time.February, "feb": time.February,
	"march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"may":  time.May,
	"june": time.June, "jun": time.June,
	"july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August,
	"september": time.September, "sep": time.September, "sept": time.September,
	"october": time.October, "oct": time.October,
	"november": time.November, "nov": time.November,
	"december": time.December, "dec": time.December,
}

// parseStartedAt converts a user-typed date string into a UTC time.
// Supported: ISO (2026-04-28 [HH:MM]), "today [HH:MM]", "yesterday [HH:MM]",
// "N days ago [HH:MM]", "28 april [HH:MM]", "28. april [HH:MM]".
// "at" is stripped before time portions.
func parseStartedAt(input string, now time.Time) (time.Time, error) {
	s := strings.ToLower(strings.TrimSpace(input))

	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t.UTC(), nil
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	parts := strings.Fields(s)

	var base time.Time
	var rest []string

	switch {
	case len(parts) >= 1 && parts[0] == "today":
		base = today
		rest = parts[1:]
	case len(parts) >= 1 && parts[0] == "yesterday":
		base = today.AddDate(0, 0, -1)
		rest = parts[1:]
	case len(parts) >= 3 && (parts[1] == "day" || parts[1] == "days") && parts[2] == "ago":
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("unrecognized format")
		}
		base = today.AddDate(0, 0, -n)
		rest = parts[3:]
	}

	if !base.IsZero() {
		return applyTime(base, rest), nil
	}

	// "DD month [HH:MM]" or "DD. month [HH:MM]"
	if len(parts) >= 2 {
		dayStr := strings.TrimSuffix(parts[0], ".")
		day, err := strconv.Atoi(dayStr)
		if err == nil {
			if m, ok := monthNames[parts[1]]; ok {
				base = time.Date(now.Year(), m, day, 0, 0, 0, 0, now.Location())
				return applyTime(base, parts[2:]), nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %q", input)
}

func applyTime(base time.Time, parts []string) time.Time {
	if len(parts) == 0 {
		return base
	}
	if parts[0] == "at" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return base
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, parts[0]); err == nil {
			return time.Date(base.Year(), base.Month(), base.Day(), t.Hour(), t.Minute(), t.Second(), 0, base.Location())
		}
	}
	return base
}
