package stats

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Today returns the UTC boundaries [from, to) that represent the current calendar
// day in the given location. from is local midnight in UTC; to is from + 24h.
func Today(loc *time.Location) (from, to time.Time) {
	now := time.Now().In(loc)
	y, m, d := now.Date()
	from = time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
	to = from.Add(24 * time.Hour)
	return
}

// ThisWeek returns [Mon 00:00, next Mon 00:00) in loc, expressed in UTC.
func ThisWeek(loc *time.Location) (from, to time.Time) {
	now := time.Now().In(loc)
	y, m, d := now.Date()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7 so Monday is 1
	}
	from = time.Date(y, m, d-weekday+1, 0, 0, 0, 0, loc).UTC()
	to = from.Add(7 * 24 * time.Hour)
	return
}

// Last7Days returns [today-7d 00:00, tomorrow 00:00) in loc, expressed in UTC.
func Last7Days(loc *time.Location) (from, to time.Time) {
	now := time.Now().In(loc)
	y, m, d := now.Date()
	tomorrow := time.Date(y, m, d+1, 0, 0, 0, 0, loc).UTC()
	from = tomorrow.Add(-7 * 24 * time.Hour)
	to = tomorrow
	return
}

// ThisMonth returns [1st of month 00:00, 1st of next month 00:00) in loc, expressed in UTC.
func ThisMonth(loc *time.Location) (from, to time.Time) {
	now := time.Now().In(loc)
	y, m, _ := now.Date()
	from = time.Date(y, m, 1, 0, 0, 0, 0, loc).UTC()
	to = time.Date(y, m+1, 1, 0, 0, 0, 0, loc).UTC()
	return
}

// ParseCustomRange parses "DD.MM.YYYY - DD.MM.YYYY" (flexible whitespace around the dash).
// Returns [from, to) as UTC midnight boundaries in loc.
// Errors if dates are unparseable, from > to, or range exceeds 366 days.
func ParseCustomRange(input string, loc *time.Location) (from, to time.Time, err error) {
	parts := strings.SplitN(strings.TrimSpace(input), "-", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, errors.New("invalid format")
	}
	fromDate, err := time.ParseInLocation("02.01.2006", strings.TrimSpace(parts[0]), loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
	}
	toDate, err := time.ParseInLocation("02.01.2006", strings.TrimSpace(parts[1]), loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
	}
	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, errors.New("start date must not be after end date")
	}
	from = fromDate.UTC()
	to = toDate.Add(24 * time.Hour).UTC()
	if to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("range exceeds 366 days")
	}
	return from, to, nil
}
