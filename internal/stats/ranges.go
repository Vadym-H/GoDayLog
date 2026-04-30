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
