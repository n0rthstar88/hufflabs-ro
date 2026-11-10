package invite

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		c, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != 10 {
			t.Fatalf("len(%q) = %d, want 10", c, len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("%q has %q outside Alphabet", c, r)
			}
		}
		if seen[c] {
			t.Fatalf("duplicate code %q", c)
		}
		seen[c] = true
	}
}

func TestAlphabetUnambiguous(t *testing.T) {
	for _, r := range "0O1IL" {
		if strings.ContainsRune(Alphabet, r) {
			t.Errorf("Alphabet contains ambiguous %q", r)
		}
	}
}

func TestHashNormalizes(t *testing.T) {
	a, b := Hash("abcd-efgh23"), Hash("ABCDEFGH23")
	if a != b {
		t.Errorf("Hash not normalized: %s vs %s", a, b)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Errorf("Hash = %q, want 64 lower hex", a)
	}
}
