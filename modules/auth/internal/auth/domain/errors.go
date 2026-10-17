package domain

import "errors"

var (
	ErrInvalidUserID       = errors.New("invalid user ID")
	ErrInvalidLogin        = errors.New("login must be 3 to 64 characters: letters, digits, '.', '_', '-', '@'")
	ErrInvalidPassword     = errors.New("password must be 8 to 72 characters long and contain a letter and a digit")
	ErrSamePassword        = errors.New("new password must differ from the current one")
	ErrInvalidCredentials  = errors.New("invalid login or password")
	ErrWrongPassword       = errors.New("current password is wrong")
	ErrCredentialsNotFound = errors.New("credentials not found")
	ErrLoginTaken          = errors.New("login is already taken")
)
