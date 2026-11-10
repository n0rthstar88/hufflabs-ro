package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hufflabs-ro/internal/manifest"
	"hufflabs-ro/internal/update"
)

func TestWatchdogCancelsStalledDownload(t *testing.T) {
	stallTimeout, stallCheck = 200*time.Millisecond, 20*time.Millisecond
	defer func() { stallTimeout, stallCheck = 60*time.Second, 5*time.Second }()
	content := []byte(strings.Repeat("x", 1000))
	sum := sha256.Sum256(content)
	release := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write(content[:10])
		w.(http.Flusher).Flush()
		<-release // never sends the rest
	}))
	defer srv.Close()
	defer close(release) // runs first, so Close isn't stuck on the stalled handler
	dir := t.TempDir()
	m := manifest.Manifest{Serial: 1, Files: []manifest.Entry{{Path: "a.bin", Size: 1000, SHA256: hex.EncodeToString(sum[:])}}}
	p, st, _ := update.MakePlan(dir, update.State{}, m, nil)
	done := make(chan error, 1)
	go func() {
		_, err := applyWithWatchdog(context.Background(), srv.Client(), srv.URL, dir, p, st, m, func(update.Progress) {})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stalled") {
			t.Errorf("err = %v, want stalled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watchdog didn't cancel the stalled download")
	}
}
