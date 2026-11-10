// Package manifest is the updater's signed file list: every file the launcher keeps
// in sync, with its size and SHA-256, plus an increasing serial. The file served to
// launchers is an envelope holding the JSON payload and ed25519 signatures over its
// exact bytes, so a server can't change it without the publisher's key.
package manifest

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrBadSignature = errors.New("manifest signature not valid")
	ErrBadManifest  = errors.New("manifest not valid")
)

// Entry is one managed file. Path uses forward slashes, relative to the install folder.
type Entry struct {
	Path   string
	Size   int64
	SHA256 string // lowercase hex
}

type Manifest struct {
	Serial  int64
	Created time.Time
	Files   []Entry
}

type sig struct {
	Key string `json:"key"`
	Sig string `json:"sig"`
}

type envelope struct {
	Payload string `json:"payload"`
	Sigs    []sig  `json:"sigs"`
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// Sign checks m, sorts its files by path and returns the envelope bytes.
func Sign(m Manifest, priv ed25519.PrivateKey) ([]byte, error) {
	m.Files = append([]Entry(nil), m.Files...)
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	if err := check(m); err != nil {
		return nil, err
	}
	payload, err := marshal(m)
	if err != nil {
		return nil, err
	}
	pub := priv.Public().(ed25519.PublicKey)
	return marshal(envelope{
		Payload: base64.StdEncoding.EncodeToString(payload),
		Sigs:    []sig{{Key: base64.StdEncoding.EncodeToString(pub), Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))}},
	})
}

// Verify accepts the envelope only if a signature from one of trusted covers the
// payload, and every entry passes the path and field checks.
func Verify(env []byte, trusted []ed25519.PublicKey) (Manifest, error) {
	var e envelope
	if err := json.Unmarshal(env, &e); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrBadManifest, err)
	}
	payload, err := base64.StdEncoding.DecodeString(e.Payload)
	if err != nil || len(payload) == 0 {
		return Manifest{}, fmt.Errorf("%w: payload", ErrBadManifest)
	}
	ok := false
	for _, s := range e.Sigs {
		sb, err := base64.StdEncoding.DecodeString(s.Sig)
		if err != nil {
			continue
		}
		for _, k := range trusted {
			if ed25519.Verify(k, payload, sb) {
				ok = true
			}
		}
	}
	if !ok {
		return Manifest{}, ErrBadSignature
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrBadManifest, err)
	}
	if err := check(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func check(m Manifest) error {
	seen := make(map[string]bool, len(m.Files))
	for _, f := range m.Files {
		if err := ValidPath(f.Path); err != nil {
			return fmt.Errorf("%w: %q: %v", ErrBadManifest, f.Path, err)
		}
		if Unmanaged(f.Path) {
			return fmt.Errorf("%w: %q is a player file", ErrBadManifest, f.Path)
		}
		key := strings.ToLower(f.Path) // Windows folders are case-insensitive
		if seen[key] {
			return fmt.Errorf("%w: %q listed twice", ErrBadManifest, f.Path)
		}
		seen[key] = true
		if h, err := hex.DecodeString(f.SHA256); err != nil || len(h) != 32 || f.SHA256 != strings.ToLower(f.SHA256) {
			return fmt.Errorf("%w: %q: bad hash", ErrBadManifest, f.Path)
		}
		if f.Size < 0 {
			return fmt.Errorf("%w: %q: bad size", ErrBadManifest, f.Path)
		}
	}
	return nil
}

// ParseKeys reads comma-separated base64 ed25519 public keys.
func ParseKeys(s string) ([]ed25519.PublicKey, error) {
	var out []ed25519.PublicKey
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(part)
		if err != nil || len(b) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("bad public key %q", part)
		}
		out = append(out, ed25519.PublicKey(b))
	}
	if len(out) == 0 {
		return nil, errors.New("no public keys")
	}
	return out, nil
}
