package storage

import "errors"

var (
	UserExists      = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)
