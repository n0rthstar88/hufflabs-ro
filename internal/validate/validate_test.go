package validate

import (
	"errors"
	"strings"
	"testing"
)

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("want *validate.Error, got %T (%v)", err, err)
	}
	return ve.Field
}

func TestUsername(t *testing.T) {
	ok := []string{"abcd", "Jim_99", strings.Repeat("a", 23)}
	for _, u := range ok {
		if err := Username(u); err != nil {
			t.Errorf("Username(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{"abc", strings.Repeat("a", 24), "jim bob", "jöse", "a-b-c", ""}
	for _, u := range bad {
		err := Username(u)
		if err == nil {
			t.Errorf("Username(%q) = nil, want error", u)
			continue
		}
		if f := fieldOf(t, err); f != "username" {
			t.Errorf("Username(%q) field = %q, want username", u, f)
		}
		if err.Error() != "Username must be 4 to 23 letters, numbers or underscores." {
			t.Errorf("Username(%q) msg = %q", u, err.Error())
		}
	}
}

func TestPassword(t *testing.T) {
	if err := Password("12345678", "12345678"); err != nil {
		t.Errorf("8 chars: %v", err)
	}
	p23 := strings.Repeat("p", 23)
	if err := Password(p23, p23); err != nil {
		t.Errorf("23 chars: %v", err)
	}
	for _, p := range []string{"1234567", strings.Repeat("p", 24)} {
		err := Password(p, p)
		if err == nil || err.Error() != "Password must be 8 to 23 characters." {
			t.Errorf("Password(len %d) = %v", len(p), err)
		}
		if err != nil && fieldOf(t, err) != "password" {
			t.Errorf("field for len %d", len(p))
		}
	}
	err := Password("12345678", "12345679")
	if err == nil || err.Error() != "Passwords don't match." {
		t.Errorf("mismatch = %v", err)
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := NormalizeCode(" abcd-efgh 23 "); got != "ABCDEFGH23" {
		t.Errorf("NormalizeCode = %q, want ABCDEFGH23", got)
	}
}
