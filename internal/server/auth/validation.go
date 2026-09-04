package auth

import (
	"errors"
	"strings"
	"unicode/utf8"

	store "github.com/erizkiatama/ansara-gym/internal/trainer"
)

const (
	minPasswordRunes = 8
	maxPasswordRunes = 128
	maxEmailBytes    = 254
)

func validateSignup(trainer store.Trainer, password string) error {
	if err := validateEmail(trainer.Email); err != nil {
		return err
	}
	if trainer.Name == "" {
		return errors.New("name is required")
	}
	return validatePassword(password)
}

func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > maxEmailBytes {
		return errors.New("invalid email")
	}
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") {
		return errors.New("invalid email")
	}
	return nil
}

func validatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordRunes || n > maxPasswordRunes {
		return errors.New("password must be 8 to 128 characters")
	}
	return nil
}
