package manifest

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const themePath = "data/texture/À¯ÀúÀÎÅÍÆäÀÌ½º/login_interface/win_login.bmp"

func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func sample() Manifest {
	return Manifest{Serial: 1762761600, Created: time.Unix(1762761600, 0).UTC(), Files: []Entry{
		{Path: "data.grf", Size: 10, SHA256: strings.Repeat("a", 64)},
		{Path: themePath, Size: 3, SHA256: strings.Repeat("b", 64)},
		{Path: "R&D.txt", Size: 0, SHA256: strings.Repeat("c", 64)},
	}}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	pubA, privA := keypair(t)
	pubB, _ := keypair(t)
	env, err := Sign(sample(), privA)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Verify(env, []ed25519.PublicKey{pubB, pubA})
	if err != nil {
		t.Fatal(err)
	}
	if m.Serial != 1762761600 || len(m.Files) != 3 || !m.Created.Equal(sample().Created) {
		t.Errorf("got %+v", m)
	}
	for i := 1; i < len(m.Files); i++ {
		if m.Files[i-1].Path > m.Files[i].Path {
			t.Errorf("files not sorted: %v", m.Files)
		}
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, privA := keypair(t)
	pubB, _ := keypair(t)
	env, _ := Sign(sample(), privA)
	if _, err := Verify(env, []ed25519.PublicKey{pubB}); !errors.Is(err, ErrBadSignature) {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	pub, priv := keypair(t)
	env, _ := Sign(sample(), priv)
	var e envelope
	if err := json.Unmarshal(env, &e); err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(e.Payload)
	raw = bytes.Replace(raw, []byte(`"Size":10`), []byte(`"Size":11`), 1)
	e.Payload = base64.StdEncoding.EncodeToString(raw)
	bad, _ := json.Marshal(e)
	if _, err := Verify(bad, []ed25519.PublicKey{pub}); !errors.Is(err, ErrBadSignature) {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}

func TestVerifyRejectsNotJSON(t *testing.T) {
	pub, priv := keypair(t)
	env, _ := Sign(sample(), priv)
	for _, in := range [][]byte{[]byte("<html>login</html>"), nil, []byte("{"), env[:len(env)-10]} {
		if _, err := Verify(in, []ed25519.PublicKey{pub}); err == nil {
			t.Errorf("Verify(%.20q) accepted", in)
		}
	}
}

func signedRaw(t *testing.T, priv ed25519.PrivateKey, m Manifest) []byte {
	t.Helper()
	// bypass Sign's checks so Verify's own checks are what's tested
	payload, _ := json.Marshal(m)
	e := envelope{Payload: base64.StdEncoding.EncodeToString(payload),
		Sigs: []sig{{Key: base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)), Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))}}}
	b, _ := json.Marshal(e)
	return b
}

func TestVerifyRejectsUnmanagedAndDuplicate(t *testing.T) {
	pub, priv := keypair(t)
	h := strings.Repeat("a", 64)
	for name, files := range map[string][]Entry{
		"unmanaged": {{Path: "savedata/x", Size: 1, SHA256: h}},
		"duplicate": {{Path: "a.txt", Size: 1, SHA256: h}, {Path: "a.txt", Size: 1, SHA256: h}},
		"bad path":  {{Path: "../evil.exe", Size: 1, SHA256: h}},
		"bad hash":  {{Path: "a.txt", Size: 1, SHA256: "XYZ"}},
		"neg size":  {{Path: "a.txt", Size: -1, SHA256: h}},
	} {
		env := signedRaw(t, priv, Manifest{Serial: 1, Files: files})
		if _, err := Verify(env, []ed25519.PublicKey{pub}); !errors.Is(err, ErrBadManifest) {
			t.Errorf("%s: err = %v, want ErrBadManifest", name, err)
		}
	}
}

func TestMojibakePathRoundTrip(t *testing.T) {
	pub, priv := keypair(t)
	env, _ := Sign(sample(), priv)
	m, err := Verify(env, []ed25519.PublicKey{pub})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range m.Files {
		if bytes.Equal([]byte(f.Path), []byte(themePath)) {
			found = true
		}
		if f.Path == "R&D.txt" {
			found = found && true
		}
	}
	if !found {
		t.Errorf("theme path not byte-identical after round trip: %v", m.Files)
	}
	if bytes.Contains(env, []byte(`\u0026`)) {
		// payload is base64 so this can only appear if escaping leaked into the envelope
		t.Error("HTML escaping in envelope")
	}
}

func TestParseKeys(t *testing.T) {
	a, _ := keypair(t)
	b, _ := keypair(t)
	s := base64.StdEncoding.EncodeToString(a) + "," + base64.StdEncoding.EncodeToString(b)
	ks, err := ParseKeys(s)
	if err != nil || len(ks) != 2 || !ks[0].Equal(a) || !ks[1].Equal(b) {
		t.Errorf("ParseKeys = %v, %v", ks, err)
	}
	for _, bad := range []string{"", "notbase64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := ParseKeys(bad); err == nil {
			t.Errorf("ParseKeys(%q) accepted", bad)
		}
	}
}
