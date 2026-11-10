package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hufflabs-ro/internal/manifest"
)

func TestFetchManifestOK(t *testing.T) {
	pub, priv := keys(t)
	env, _ := manifest.Sign(manifest.Manifest{Serial: 7, Files: []manifest.Entry{entry("a.txt", []byte("a"))}}, priv)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/update/manifest.json" || r.Header.Get("Cache-Control") != "no-cache" {
			http.NotFound(w, r)
			return
		}
		w.Write(env)
	}))
	defer srv.Close()
	m, err := FetchManifest(context.Background(), srv.Client(), srv.URL, []ed25519.PublicKey{pub})
	if err != nil || m.Serial != 7 {
		t.Errorf("FetchManifest = %+v, %v", m, err)
	}
}

func TestFetchManifestNonManifest(t *testing.T) {
	pub, priv := keys(t)
	env, _ := manifest.Sign(manifest.Manifest{Serial: 7}, priv)
	for name, h := range map[string]http.HandlerFunc{
		"404":       func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"html":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>Tailscale login</html>")) },
		"truncated": func(w http.ResponseWriter, r *http.Request) { w.Write(env[:len(env)/2]) },
		"huge":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat(" ", 20<<20))) },
	} {
		srv := httptest.NewTLSServer(h)
		_, err := FetchManifest(context.Background(), srv.Client(), srv.URL, []ed25519.PublicKey{pub})
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
		srv.Close()
	}
	// a non-https site URL is refused outright
	if _, err := FetchManifest(context.Background(), http.DefaultClient, "http://example.invalid", []ed25519.PublicKey{pub}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("http URL err = %v", err)
	}
}
