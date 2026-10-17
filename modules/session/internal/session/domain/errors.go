package domain

import "errors"

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidToken    = errors.New("invalid session token")
	ErrInvalidUserID   = errors.New("invalid user ID")
	ErrInvalidPolicy   = errors.New("invalid session policy")
)
