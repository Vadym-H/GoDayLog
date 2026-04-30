package tghandlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/services"
	"github.com/Vadym-H/GoDayLog/internal/stats"
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

		// day/week breakdown for multi-day ranges
		// ByDay from DB only contains days with activities, so no empty rows appear.
		if len(report.ByDay) > 0 {
			sb.WriteString("\n\n─────────────")

			if report.To.Sub(report.From) > 14*24*time.Hour {
				// weekly buckets: group ByDay by Mon, aggregate, one line per week
				type weekBucket struct {
					monday time.Time
					sunday time.Time
					byType map[string]services.TypeSummary
				}
				var weekOrder []time.Time
				weekMap := make(map[time.Time]*weekBucket)

				for _, ds := range report.ByDay {
					wd := int(ds.Date.Weekday())
					if wd == 0 {
						wd = 7
					}
					monday := ds.Date.AddDate(0, 0, 1-wd)
					wb, exists := weekMap[monday]
					if !exists {
						wb = &weekBucket{
							monday: monday,
							sunday: monday.AddDate(0, 0, 6),
							byType: make(map[string]services.TypeSummary),
						}
						weekMap[monday] = wb
						weekOrder = append(weekOrder, monday)
					}
					for _, ts := range ds.ByType {
						agg := wb.byType[ts.Type]
						agg.Type = ts.Type
						agg.Count += ts.Count
						agg.TotalMinutes += ts.TotalMinutes
						wb.byType[ts.Type] = agg
					}
				}

				for _, monday := range weekOrder {
					wb := weekMap[monday]
					var dateLabel string
					if monday.Month() == wb.sunday.Month() {
						dateLabel = fmt.Sprintf("%d–%d %s", monday.Day(), wb.sunday.Day(), monday.Format("Jan"))
					} else {
						dateLabel = fmt.Sprintf("%d %s–%d %s", monday.Day(), monday.Format("Jan"), wb.sunday.Day(), wb.sunday.Format("Jan"))
					}
					typeSummaries := make([]services.TypeSummary, 0, len(wb.byType))
					for _, ts := range wb.byType {
						typeSummaries = append(typeSummaries, ts)
					}
					sb.WriteString("\n")
					sb.WriteString(fmt.Sprintf("%-14s  %s", dateLabel, formatDayLine(services.DaySummary{ByType: typeSummaries})))
				}
			} else {
				// daily: iterate ByDay directly — DB only returns days with activities
				for _, ds := range report.ByDay {
					sb.WriteString("\n")
					sb.WriteString(fmt.Sprintf("%-12s  %s", ds.Date.Format("Mon 2 Jan"), formatDayLine(ds)))
				}
			}
		}
	}

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Change range", CallbackData: callbackStatsPicker},
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

func (tg *TgHandlers) HandlePendingStatsRangeInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil {
		return false
	}
	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingStatsRange(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingStatsRange(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Send a date range, e.g. `20.01.2025 - 20.05.2025`",
		})
		if err != nil {
			log.Error("failed to re-prompt stats range", slog.Any("error", err))
		}
		return true
	}

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}

	tzName, err := tg.userService.GetUserTimezone(ctx, identity)
	if err != nil {
		log.Error("failed to get timezone", slog.Any("error", err))
		tzName = "UTC"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}

	from, to, parseErr := stats.ParseCustomRange(text, loc)
	if parseErr != nil {
		tg.setAwaitingStatsRange(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Could not parse range: %s\nExample: `20.01.2025 - 20.05.2025`", parseErr.Error()),
		})
		if err != nil {
			log.Error("failed to send parse error", slog.Any("error", err))
		}
		return true
	}

	userID, err := tg.userService.GetUserID(ctx, identity)
	if err != nil {
		log.Error("failed to get user id", slog.Any("error", err))
		return true
	}

	report, err := tg.statsService.GetStats(ctx, services.StatsQuery{
		UserID:   userID,
		From:     from,
		To:       to,
		Timezone: tzName,
	})
	if err != nil {
		log.Error("failed to get stats", slog.Any("error", err))
		return true
	}

	label := from.In(loc).Format("2 Jan 2006") + " – " + to.In(loc).Add(-time.Second).Format("2 Jan 2006")
	if err := tg.sendStatsMessage(ctx, bot, chatID, report, label, loc); err != nil {
		log.Error("failed to send stats message", slog.Any("error", err))
	}
	return true
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
