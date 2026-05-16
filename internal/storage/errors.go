package storage

import "errors"

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidMessageStatus = errors.New("invalid message status")
	ErrMessageInUse         = errors.New("message is referenced by activity logs")
	ErrMessageFailed        = errors.New("message has failed status")
	ErrActivityNotFound     = errors.New("activity not found")
)
