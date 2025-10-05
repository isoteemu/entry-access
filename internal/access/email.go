package access

import (
	"errors"
	"strings"
)

var (
	ErrMissingEmail = errors.New("email is required")
	ErrInvalidEmail = errors.New("invalid email format")
)

func ValidEmail(email string) error {
	// TODO: Validate from AD/LDAP if configured

	// A very basic check for email format
	// Basic validation
	if email == "" {
		return ErrMissingEmail
	}

	// Must contain "@" and not be the first or last character
	at := strings.Index(email, "@")
	if at < 1 || at == len(email)-1 {
		return ErrInvalidEmail
	}

	return nil
}
