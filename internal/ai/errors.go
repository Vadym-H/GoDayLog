package ai

import "errors"

// ErrInputTooLong is returned by ExtractActivity when the estimated input token
// count exceeds the configured limit. No HTTP call was made; no tokens were billed.
var ErrInputTooLong = errors.New("ai: input too long")

// ErrDailyBudgetExceeded is returned when the user has consumed their daily token budget.
var ErrDailyBudgetExceeded = errors.New("ai: daily token budget exceeded")

var ErrAudioTooLarge = errors.New("ai: audio too large")
var ErrAudioBudgetExceeded = errors.New("ai: daily audio budget exceeded")
var ErrUnsupportedAudioType = errors.New("ai: unsupported audio type")
