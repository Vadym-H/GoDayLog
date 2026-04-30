package stats

import "time"

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
