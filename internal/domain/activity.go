package domain

import "time"

type Activity struct {
	Description     string
	Tag             string
	ActivityType    string
	DurationMinutes *int
	StartedAt       *time.Time
	CompletedAt     *time.Time
}
