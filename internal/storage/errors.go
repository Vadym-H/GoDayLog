package storage

import "errors"

var (
	UserExists              = errors.New("user already exists")
	ErrUserNotFound         = errors.New("user not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidMessageStatus = errors.New("invalid message status")
	ErrMessageInUse         = errors.New("message is referenced by activity logs")
)
