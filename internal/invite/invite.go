// Package invite makes single-use invite codes and the hashes stored for them.
package invite

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"hufflabs-ro/internal/validate"
)

// Alphabet has no 0/O/1/I/L so codes can be read out loud or retyped.
const Alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const codeLen = 10

// New returns a random 10-character code from Alphabet.
func New() (string, error) {
	out := make([]byte, 0, codeLen)
	buf := make([]byte, 32)
	// Rejection sampling: accept bytes below the largest multiple of len(Alphabet).
	limit := byte(256 - 256%len(Alphabet))
	for len(out) < codeLen {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b < limit && len(out) < codeLen {
				out = append(out, Alphabet[int(b)%len(Alphabet)])
			}
		}
	}
	return string(out), nil
}

// Hash is the lower-hex SHA-256 of the normalized code.
func Hash(code string) string {
	sum := sha256.Sum256([]byte(validate.NormalizeCode(code)))
	return hex.EncodeToString(sum[:])
}
