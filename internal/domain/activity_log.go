package domain

import "time"

type ActivityLog struct {
	ID              string
	MessageID       string
	Description     string
	Tag             string
	ActivityType    string
	DurationMinutes *int
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

type UpdateActivityFields struct {
	Description     *string
	Tag             *string
	ActivityType    *string
	DurationMinutes *int
	StartedAt       *time.Time
}
