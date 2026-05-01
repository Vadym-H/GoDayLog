package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type ActivityEntry struct {
	Description     string
	Tag             string
	ActivityType    string
	DurationMinutes *int
	StartedAt       *time.Time
}

type ActivityEntryRepository interface {
	GetActivityEntries(ctx context.Context, q StatsQuery) ([]ActivityEntry, error)
}

type AIAnalysisClient interface {
	AnalyseStats(ctx context.Context, prompt string) (ai.AnalysisResponse, error)
}

type StatsAILogRepository interface {
	CreateStatsAILog(ctx context.Context, userID, model string, usage ai.TokenUsage) error
}

type StatsAnalyser struct {
	log       *slog.Logger
	repo      ActivityEntryRepository
	ai        AIAnalysisClient
	aiLogRepo StatsAILogRepository
}

func NewStatsAnalyser(log *slog.Logger, repo ActivityEntryRepository, aiClient AIAnalysisClient, aiLogRepo StatsAILogRepository) *StatsAnalyser {
	return &StatsAnalyser{log: log, repo: repo, ai: aiClient, aiLogRepo: aiLogRepo}
}

func (a *StatsAnalyser) Analyse(ctx context.Context, q StatsQuery, report StatsReport, userContext string) (string, error) {
	log := logger.From(ctx, a.log)

	entries, err := a.repo.GetActivityEntries(ctx, q)
	if err != nil {
		log.Error("failed to get activity entries", slog.Any("error", err))
		return "", fmt.Errorf("stats analysis: %w", err)
	}

	prompt := buildAnalysisPrompt(q, report, entries, userContext)

	result, err := a.ai.AnalyseStats(ctx, prompt)
	// Log usage before returning error: client may return partial usage even on failure (e.g. empty choices).
	if result.Usage.TotalTokens > 0 {
		if logErr := a.aiLogRepo.CreateStatsAILog(ctx, q.UserID, result.Model, result.Usage); logErr != nil {
			log.Warn("failed to log stats ai request usage", slog.Any("error", logErr))
		}
	}
	if err != nil {
		log.Error("ai analysis failed", slog.Any("error", err))
		return "", fmt.Errorf("stats analysis: %w", err)
	}

	return result.Text, nil
}

func buildAnalysisPrompt(q StatsQuery, report StatsReport, entries []ActivityEntry, userContext string) string {
	var sb strings.Builder

	loc := time.UTC
	if q.Timezone != "" {
		if l, err := time.LoadLocation(q.Timezone); err == nil {
			loc = l
		}
	}

	from := q.From.In(loc)
	toDisplay := q.To.In(loc).Add(-time.Second)

	if from.Format("2006-01-02") == toDisplay.Format("2006-01-02") {
		sb.WriteString("Period: " + from.Format("2 Jan 2006"))
	} else {
		sb.WriteString("Period: " + from.Format("2 Jan 2006") + " – " + toDisplay.Format("2 Jan 2006"))
	}

	now := time.Now().In(loc)
	sb.WriteString("\nCurrent time: " + now.Format("2 Jan 2006 15:04") + " (" + loc.String() + ")")

	sb.WriteString("\n\nBy type:\n")

	typeOrder := []string{"growth", "routine", "rest", "drain"}
	typeMap := make(map[string]TypeSummary, 4)
	for _, ts := range report.ByType {
		typeMap[ts.Type] = ts
	}

	for _, t := range typeOrder {
		ts := typeMap[t]
		line := fmt.Sprintf("  %s: %d %s", t, ts.Count, pluralActivity(ts.Count))
		if ts.TotalMinutes > 0 {
			line += " · " + analysisFmtMins(ts.TotalMinutes)
		}
		if ts.UntrackedCount > 0 {
			line += fmt.Sprintf(" (%d untracked)", ts.UntrackedCount)
		}
		sb.WriteString(line + "\n")
	}

	const maxEntries = 100
	total := len(entries)
	if total > maxEntries {
		entries = entries[total-maxEntries:]
	}

	sb.WriteString("\nActivities:\n")
	for _, e := range entries {
		typeLabel := e.ActivityType
		if e.Tag != "" && e.Tag != e.ActivityType {
			typeLabel = e.ActivityType + " | " + e.Tag
		}

		dur := "no duration"
		if e.DurationMinutes != nil {
			dur = analysisFmtMins(*e.DurationMinutes)
		}

		line := fmt.Sprintf("- [%s]  %s — %s", typeLabel, e.Description, dur)
		if e.StartedAt != nil {
			line += "  (" + e.StartedAt.In(loc).Format("15:04") + ")"
		}
		sb.WriteString(line + "\n")
	}

	if total > maxEntries {
		sb.WriteString(fmt.Sprintf("\n(showing %d of %d activities)", maxEntries, total))
	}

	if userContext != "" {
		sb.WriteString("\n\nUser context: " + userContext)
	}

	return sb.String()
}

func analysisFmtMins(m int) string {
	h := m / 60
	rem := m % 60
	if h == 0 {
		return fmt.Sprintf("%dmin", rem)
	}
	if rem == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dmin", h, rem)
}

func pluralActivity(n int) string {
	if n == 1 {
		return "activity"
	}
	return "activities"
}
