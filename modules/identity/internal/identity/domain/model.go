package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

const (
	maxNameLength  = 100
	maxEmailLength = 254
)

type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
)

type Profile struct {
	FirstName string
	LastName  string
	Email     string
}

func NewProfile(firstName, lastName, email string) (Profile, error) {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	email = strings.ToLower(strings.TrimSpace(email))
	if !validName(firstName) {
		return Profile{}, ErrInvalidFirstName
	}
	if !validName(lastName) {
		return Profile{}, ErrInvalidLastName
	}
	if !validEmail(email) {
		return Profile{}, ErrInvalidEmail
	}
	return Profile{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
	}, nil
}

func validName(name string) bool {
	length := utf8.RuneCountInString(name)
	return length > 0 && length <= maxNameLength
}

func validEmail(email string) bool {
	if len(email) > maxEmailLength {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && address.Name == ""
}

type User struct {
	ID        uuid.UUID
	Profile   Profile
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewUser(id uuid.UUID, profile Profile, now time.Time) (User, error) {
	if id == uuid.Nil() {
		return User{}, ErrInvalidUserID
	}
	now = now.UTC()
	return User{
		ID:        id,
		Profile:   profile,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (u *User) UpdateProfile(profile Profile, now time.Time) bool {
	if u.Profile == profile {
		return false
	}
	u.Profile = profile
	u.UpdatedAt = now.UTC()
	return true
}

type UserRepository interface {
	Find(id uuid.UUID) (User, error)
	FindForUpdate(id uuid.UUID) (User, error)
	Create(User) error
	Update(User) error
}
