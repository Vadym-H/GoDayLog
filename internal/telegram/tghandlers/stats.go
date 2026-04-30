package tghandlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/services"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	callbackStatsToday  = callbackActionPrefix + "stats_today"
	callbackStatsWeek   = callbackActionPrefix + "stats_week"
	callbackStatsLast7  = callbackActionPrefix + "stats_last7"
	callbackStatsMonth  = callbackActionPrefix + "stats_month"
	callbackStatsCustom = callbackActionPrefix + "stats_custom"
)

// renderBar returns a 7-character ASCII bar proportional to minutes/totalMinutes.
// Only tracked minutes count; untracked activities do not affect the bar.
func renderBar(minutes, totalMinutes int) string {
	const width = 7
	if totalMinutes == 0 || minutes == 0 {
		return strings.Repeat("░", width)
	}
	// integer rounding: (minutes * width * 2 + totalMinutes) / (2 * totalMinutes)
	filled := (minutes*width*2 + totalMinutes) / (2 * totalMinutes)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (tg *TgHandlers) sendStatsPicker(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Today", CallbackData: callbackStatsToday},
				{Text: "This Week", CallbackData: callbackStatsWeek},
			},
			{
				{Text: "Last 7 Days", CallbackData: callbackStatsLast7},
				{Text: "This Month", CallbackData: callbackStatsMonth},
			},
			{
				{Text: "Custom range", CallbackData: callbackStatsCustom},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Choose a time range:",
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) sendStatsMessage(ctx context.Context, bot *tgbot.Bot, chatID int64, report services.StatsReport, rangeLabel string, loc *time.Location) error {
	var sb strings.Builder
	sb.WriteString(rangeLabel)
	sb.WriteString("\n\n")

	if report.TotalCount == 0 {
		sb.WriteString("No activities logged.")
	} else {
		// summary line
		parts := []string{activityWord(report.TotalCount)}
		if report.TotalMinutes > 0 {
			parts = append(parts, fmtMinutes(report.TotalMinutes)+" tracked")
		}
		if report.UntrackedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", report.UntrackedCount))
		}
		sb.WriteString(strings.Join(parts, " · "))

		// type rows — always show all 4 types
		typeOrder := []string{"growth", "routine", "rest", "drain"}
		typeMap := make(map[string]services.TypeSummary, 4)
		for _, ts := range report.ByType {
			typeMap[ts.Type] = ts
		}

		sb.WriteString("\n\n")
		for i, t := range typeOrder {
			if i > 0 {
				sb.WriteString("\n")
			}
			ts := typeMap[t] // zero value (Count=0) if type absent
			bar := renderBar(ts.TotalMinutes, report.TotalMinutes)

			durPct := "—"
			if ts.TotalMinutes > 0 {
				pct := (ts.TotalMinutes * 100) / report.TotalMinutes
				durPct = fmt.Sprintf("%s (%d%%)", fmtMinutes(ts.TotalMinutes), pct)
			}

			sb.WriteString(fmt.Sprintf("%-7s  %s  %s  · %s", t, bar, durPct, activityWord(ts.Count)))
			if ts.UntrackedCount > 0 {
				sb.WriteString(fmt.Sprintf(" (%d untracked)", ts.UntrackedCount))
			}
		}

		// tags
		if len(report.TopTags) > 0 {
			sb.WriteString("\n\nTags: ")
			for i, tag := range report.TopTags {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(tag.Tag)
				if tag.Count > 1 {
					sb.WriteString(fmt.Sprintf(" ×%d", tag.Count))
				}
			}
		}

		// day breakdown for multi-day ranges
		if len(report.ByDay) > 0 {
			// index ByDay by local date (pgx returns DATE as UTC-midnight of the local date)
			byDayMap := make(map[time.Time]services.DaySummary, len(report.ByDay))
			for _, ds := range report.ByDay {
				byDayMap[ds.Date] = ds
			}

			sb.WriteString("\n\n─────────────")

			localStart := report.From.In(loc)
			y, m, d := localStart.Date()
			for i := 0; ; i++ {
				dayLocal := time.Date(y, m, d+i, 0, 0, 0, 0, loc)
				if !dayLocal.Before(report.To.In(loc)) {
					break
				}
				// key must match the format pgx returns for DATE: UTC midnight of the local date
				dayKey := time.Date(dayLocal.Year(), dayLocal.Month(), dayLocal.Day(), 0, 0, 0, 0, time.UTC)

				sb.WriteString("\n")
				sb.WriteString(dayLocal.Format("Mon 2 Jan"))
				sb.WriteString("   ")
				if ds, ok := byDayMap[dayKey]; ok {
					sb.WriteString(formatDayLine(ds))
				} else {
					sb.WriteString("—")
				}
			}
		}
	}

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Change range", CallbackData: callbackTodayStats},
			},
			{
				{Text: "Home", CallbackData: callbackHome},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        sb.String(),
		ReplyMarkup: markup,
	})
	return err
}

// formatDayLine renders the per-type breakdown for one day, e.g. "growth 45min · drain 1h".
func formatDayLine(ds services.DaySummary) string {
	order := []string{"growth", "routine", "rest", "drain"}
	typeMap := make(map[string]services.TypeSummary, len(ds.ByType))
	for _, ts := range ds.ByType {
		typeMap[ts.Type] = ts
	}

	var parts []string
	for _, t := range order {
		ts, ok := typeMap[t]
		if !ok || ts.Count == 0 {
			continue
		}
		part := t
		if ts.TotalMinutes > 0 {
			part += " " + fmtMinutes(ts.TotalMinutes)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}
