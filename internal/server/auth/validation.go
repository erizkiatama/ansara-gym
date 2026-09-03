package auth

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	minPasswordRunes = 8
	maxPasswordRunes = 128
	maxNameRunes     = 100
	maxEmailBytes    = 254
)

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > maxEmailBytes {
		return "", errors.New("invalid email")
	}
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxNameRunes {
		return "", errors.New("invalid name")
	}
	return name, nil
}

func validatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordRunes || n > maxPasswordRunes {
		return errors.New("invalid password")
	}
	return nil
}
