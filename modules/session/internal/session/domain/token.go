package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const tokenBytes = 32

type Token string

func NewToken() (Token, error) {
	b := make([]byte, tokenBytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return Token(base64.RawURLEncoding.EncodeToString(b)), nil
}

func ParseToken(s string) (Token, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) != tokenBytes {
		return "", ErrInvalidToken
	}
	return Token(s), nil
}

func (t Token) SessionID() string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}
