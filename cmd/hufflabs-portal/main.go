// Command hufflabs-portal serves the HuffLabs RO sign-up site on 127.0.0.1:8080.
// `tailscale serve` publishes it to the tailnet; nothing listens on the LAN.
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hufflabs-ro/internal/limit"
	"hufflabs-ro/internal/store"
	"hufflabs-ro/internal/tsapi"
	"hufflabs-ro/internal/web"
)

const listenAddr = "127.0.0.1:8080"

type config struct {
	portalDSN, gameDSN       string
	tsID, tsSecret, deviceID string
	web                      web.Config
}

func loadConfig(getenv func(string) string) (config, error) {
	var missing []string
	get := func(k string) string {
		v := strings.TrimSpace(getenv(k))
		if v == "" {
			missing = append(missing, k)
		}
		return v
	}
	c := config{
		portalDSN: get("PORTAL_DSN"), gameDSN: get("GAME_DSN"),
		tsID: get("TS_CLIENT_ID"), tsSecret: get("TS_CLIENT_SECRET"), deviceID: get("TS_DEVICE_ID"),
	}
	c.web.SiteURL = strings.TrimRight(get("SITE_URL"), "/")
	c.web.DownloadDir = get("DOWNLOAD_DIR")
	key := get("SESSION_KEY")
	if len(missing) > 0 {
		return c, fmt.Errorf("missing settings in portal.env: %s", strings.Join(missing, ", "))
	}
	k, err := hex.DecodeString(key)
	if err != nil || len(k) < 32 {
		return c, errors.New("SESSION_KEY must be at least 64 hex characters (openssl rand -hex 32)")
	}
	c.web.SessionKey = k
	c.web.UpdateDir = strings.TrimSpace(getenv("UPDATE_DIR"))
	c.web.ClientVersion = strings.TrimSpace(getenv("CLIENT_VERSION"))
	if c.web.ClientVersion == "" {
		if b, err := os.ReadFile(filepath.Join(c.web.DownloadDir, "VERSION")); err == nil {
			c.web.ClientVersion = strings.TrimSpace(string(b))
		}
	}
	return c, nil
}

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(2)
	}
	st, err := store.Open(cfg.portalDSN, cfg.gameDSN)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	inv := tsapi.New("https://api.tailscale.com", cfg.tsID, cfg.tsSecret, cfg.deviceID)
	lim := limit.NewLimiter(5, 10*time.Minute, time.Now)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           web.New(cfg.web, st, inv, lim),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	slog.Info("listening", "addr", listenAddr, "site", cfg.web.SiteURL, "client", cfg.web.ClientVersion)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
