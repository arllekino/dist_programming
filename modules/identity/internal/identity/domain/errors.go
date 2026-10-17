package domain

import "errors"

var (
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrInvalidFirstName  = errors.New("first name must contain 1 to 100 characters")
	ErrInvalidLastName   = errors.New("last name must contain 1 to 100 characters")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)
