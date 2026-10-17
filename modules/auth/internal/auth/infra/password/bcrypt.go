package password

import (
	"golang.org/x/crypto/bcrypt"

	"auth/internal/auth/domain"
)

func NewBcryptHasher(cost int) domain.PasswordHasher {
	return bcryptHasher{cost: cost}
}

type bcryptHasher struct {
	cost int
}

func (h bcryptHasher) Hash(password domain.Password) (domain.PasswordHash, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return domain.PasswordHash(hash), nil
}

func (h bcryptHasher) Matches(hash domain.PasswordHash, candidate string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(candidate)) == nil
}
