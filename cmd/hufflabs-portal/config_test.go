package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func full(dl string) map[string]string {
	return map[string]string{
		"PORTAL_DSN": "p", "GAME_DSN": "g", "TS_CLIENT_ID": "id", "TS_CLIENT_SECRET": "sec", "TS_DEVICE_ID": "dev",
		"SITE_URL": "https://ro.example.ts.net", "DOWNLOAD_DIR": dl,
		"SESSION_KEY": strings.Repeat("ab", 32),
	}
}

func TestLoadConfigMissingNamesAll(t *testing.T) {
	_, err := loadConfig(envFrom(map[string]string{"PORTAL_DSN": "p"}))
	if err == nil {
		t.Fatal("want error")
	}
	for _, k := range []string{"GAME_DSN", "TS_CLIENT_ID", "TS_CLIENT_SECRET", "TS_DEVICE_ID", "SITE_URL", "DOWNLOAD_DIR", "SESSION_KEY"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q doesn't name %s", err, k)
		}
	}
}

func TestLoadConfigBadSessionKey(t *testing.T) {
	m := full(t.TempDir())
	m["SESSION_KEY"] = "short"
	if _, err := loadConfig(envFrom(m)); err == nil || !strings.Contains(err.Error(), "SESSION_KEY") {
		t.Errorf("err = %v", err)
	}
}

func TestLoadConfigVersionFromFile(t *testing.T) {
	dl := t.TempDir()
	os.WriteFile(filepath.Join(dl, "VERSION"), []byte("2026.11.09\n"), 0o644)
	c, err := loadConfig(envFrom(full(dl)))
	if err != nil {
		t.Fatal(err)
	}
	if c.web.ClientVersion != "2026.11.09" || len(c.web.SessionKey) != 32 || c.web.SiteURL != "https://ro.example.ts.net" {
		t.Errorf("cfg = %+v", c.web)
	}
}

func TestLoadConfigUpdateDirOptional(t *testing.T) {
	m := full(t.TempDir())
	c, err := loadConfig(envFrom(m))
	if err != nil || c.web.UpdateDir != "" {
		t.Fatalf("without UPDATE_DIR: %+v, %v", c.web.UpdateDir, err)
	}
	m["UPDATE_DIR"] = "/srv/hufflabs-portal/update"
	c, _ = loadConfig(envFrom(m))
	if c.web.UpdateDir != "/srv/hufflabs-portal/update" {
		t.Errorf("UpdateDir = %q", c.web.UpdateDir)
	}
}
