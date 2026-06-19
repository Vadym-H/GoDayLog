package domain

import "time"

type StatsQuery struct {
	UserID   string
	From     time.Time
	To       time.Time
	Timezone string
}

type TypeSummary struct {
	Type           string
	Count          int
	TotalMinutes   int
	UntrackedCount int
}

type TagSummary struct {
	Tag          string
	Count        int
	TotalMinutes int
}

type DaySummary struct {
	Date         time.Time
	Count        int
	TotalMinutes int
	ByType       []TypeSummary
}

type StatsReport struct {
	From            time.Time
	To              time.Time
	FirstActivityAt *time.Time
	LastActivityAt  *time.Time
	TotalCount      int
	TotalMinutes    int
	UntrackedCount  int
	ByType          []TypeSummary
	TopTags         []TagSummary
	ByDay           []DaySummary
}
