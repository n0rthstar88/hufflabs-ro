// Package validate holds the sign-up rules for HuffLabs RO accounts and invite codes.
package validate

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Error is a validation failure. Msg is shown to users verbatim.
type Error struct {
	Field string
	Msg   string
}

func (e *Error) Error() string { return e.Msg }

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{4,23}$`)

// Username accepts 4–23 ASCII letters, digits or underscores.
func Username(s string) error {
	if !usernameRe.MatchString(s) {
		return &Error{Field: "username", Msg: "Username must be 4 to 23 letters, numbers or underscores."}
	}
	return nil
}

// Password accepts 8–23 characters (rAthena truncates longer ones) that match confirm.
func Password(pw, confirm string) error {
	if n := utf8.RuneCountInString(pw); n < 8 || n > 23 || len(pw) > 23 {
		return &Error{Field: "password", Msg: "Password must be 8 to 23 characters."}
	}
	if pw != confirm {
		return &Error{Field: "confirm", Msg: "Passwords don't match."}
	}
	return nil
}

// NormalizeCode upper-cases an invite code and strips spaces and dashes.
func NormalizeCode(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '\t' {
			return -1
		}
		return r
	}, strings.ToUpper(s))
}
