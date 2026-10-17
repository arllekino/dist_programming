package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"uuid"
)

var loginPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]{2,63}$`)

type Login string

func NewLogin(s string) (Login, error) {
	login := strings.ToLower(strings.TrimSpace(s))
	if !loginPattern.MatchString(login) {
		return "", ErrInvalidLogin
	}
	return Login(login), nil
}

const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

type Password string

func NewPassword(password string) (Password, error) {
	err := validatePassword(password)
	if err != nil {
		return "", err
	}
	return Password(password), nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return ErrInvalidPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		hasLetter = hasLetter || unicode.IsLetter(r)
		hasDigit = hasDigit || unicode.IsDigit(r)
	}
	if !hasLetter || !hasDigit {
		return ErrInvalidPassword
	}
	return nil
}

type PasswordHash string

type PasswordHasher interface {
	Hash(Password) (PasswordHash, error)
	Matches(hash PasswordHash, candidate string) bool
}

type Credentials struct {
	UserID            uuid.UUID
	Login             Login
	PasswordHash      PasswordHash
	CreatedAt         time.Time
	PasswordChangedAt time.Time
}

func NewCredentials(userID uuid.UUID, login Login, hash PasswordHash, now time.Time) (Credentials, error) {
	if userID == uuid.Nil() {
		return Credentials{}, ErrInvalidUserID
	}
	now = now.UTC()
	return Credentials{
		UserID:            userID,
		Login:             login,
		PasswordHash:      hash,
		CreatedAt:         now,
		PasswordChangedAt: now,
	}, nil
}

func (c Credentials) VerifyPassword(hasher PasswordHasher, candidate string) bool {
	return hasher.Matches(c.PasswordHash, candidate)
}

func (c *Credentials) ChangePassword(hash PasswordHash, now time.Time) {
	c.PasswordHash = hash
	c.PasswordChangedAt = now.UTC()
}

type CredentialsRepository interface {
	FindByLogin(Login) (Credentials, error)
	FindByUserID(uuid.UUID) (Credentials, error)
	FindByUserIDForUpdate(uuid.UUID) (Credentials, error)
	Create(Credentials) error
	Update(Credentials) error
}
