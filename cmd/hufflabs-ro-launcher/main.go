// Command hufflabs-ro-launcher keeps a HuffLabs RO install up to date and starts
// the game. Build settings (see launcher/build.sh):
//
//	-X main.siteURL=https://…   the HuffLabs RO site
//	-X main.trustedKeys=<b64>,<b64>   ed25519 public keys allowed to sign updates
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hufflabs-ro/internal/manifest"
	"hufflabs-ro/internal/update"
)

var siteURL, trustedKeys string

var stallTimeout, stallCheck = 60 * time.Second, 5 * time.Second

func main() {
	demo := flag.String("demo", "", "show a sample screen (checking, scanning, downloading, offline, starting) and change nothing")
	flag.Bool("updated", false, "set when the launcher restarts itself after an update")
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir := filepath.Dir(exe)
	setupLog(dir)

	if *demo != "" {
		v, ok := demoViews()[*demo]
		if !ok {
			fmt.Fprintln(os.Stderr, "unknown demo screen")
			os.Exit(2)
		}
		runUI(context.Background(), func(ctx context.Context, show func(View), choices <-chan Choice) Result {
			show(v)
			select {
			case <-choices:
			case <-ctx.Done():
			}
			return Result{}
		})
		return
	}

	site := siteOverride(dir, siteURL)
	keys, kerr := manifest.ParseKeys(trustedKeys)
	if kerr != nil {
		slog.Error("trusted keys", "err", kerr)
	}
	client := &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
	}}
	deps := Deps{
		Configured: site != "" && kerr == nil,
		Installed:  isInstallFolder(dir),
		SelfRel:    filepath.Base(exe),
		Lock:       func() (func(), error) { return update.Lock(dir) },
		CleanupOld: func() { update.CleanupOld(exe) },
		Fetch: func(ctx context.Context) (manifest.Manifest, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			return update.FetchManifest(ctx, client, site, keys)
		},
		LoadState: func() (update.State, error) { return update.LoadState(dir) },
		MakePlan: func(st update.State, m manifest.Manifest, progress func(int, int)) (update.Plan, update.State, error) {
			return update.MakePlan(dir, st, m, progress)
		},
		Apply: func(ctx context.Context, p update.Plan, st update.State, m manifest.Manifest, report func(update.Progress)) (update.State, error) {
			return applyWithWatchdog(ctx, client, site, dir, p, st, m, report)
		},
	}
	var game *exec.Cmd
	deps.StartGame = func() error {
		cmd, err := startGame(dir)
		game = cmd
		return err
	}

	slog.Info("launcher start", "dir", dir, "site", site)
	res := runUI(context.Background(), func(ctx context.Context, show func(View), choices <-chan Choice) Result {
		return Run(ctx, deps, show, choices)
	})
	slog.Info("launcher done", "started", res.Started, "reexec", res.Reexec)
	switch {
	case res.Reexec:
		slog.Info("restarting after self-update")
		if err := exec.Command(exe, "--updated").Start(); err != nil {
			slog.Error("restart", "err", err)
		}
	case res.Started && game != nil:
		// hold the lock while the game runs, so a second launch can't update files
		// the game has open
		release, err := update.Lock(dir)
		game.Wait()
		if err == nil {
			release()
		}
	}
}

// applyWithWatchdog cancels the update if no bytes move for stallTimeout, so a
// dead connection turns into a "Retry" instead of a frozen bar.
func applyWithWatchdog(ctx context.Context, c *http.Client, site, dir string, p update.Plan, st update.State, m manifest.Manifest, report func(update.Progress)) (update.State, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var mu sync.Mutex
	last := time.Now()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(stallCheck)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				mu.Lock()
				idle := time.Since(last)
				mu.Unlock()
				if idle > stallTimeout {
					cancel(fmt.Errorf("download stalled for %s", stallTimeout))
					return
				}
			}
		}
	}()
	st, err := update.Apply(ctx, c, site, dir, p, st, m, func(pr update.Progress) {
		mu.Lock()
		last = time.Now()
		mu.Unlock()
		report(pr)
	})
	if cause := context.Cause(ctx); err != nil && cause != nil && cause != context.Canceled {
		err = cause
	}
	return st, err
}

// isInstallFolder reports whether dir holds a HuffLabs RO install (or one the
// launcher already manages), so a copied-away launcher doesn't fill a random
// folder with game files.
func isInstallFolder(dir string) bool {
	for _, f := range []string{filepath.Join(update.StateDir, "state.json"), gameExe, "data.grf"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

// siteOverride reads "site=https://…" from launcher.cfg next to the exe, if any.
func siteOverride(dir, def string) string {
	f, err := os.Open(filepath.Join(dir, "launcher.cfg"))
	if err != nil {
		return def
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if ok && strings.TrimSpace(k) == "site" && strings.HasPrefix(strings.TrimSpace(v), "https://") {
			return strings.TrimRight(strings.TrimSpace(v), "/")
		}
	}
	return def
}

func setupLog(dir string) {
	path := filepath.Join(dir, update.StateDir, "launcher.log")
	os.MkdirAll(filepath.Dir(path), 0o755)
	if fi, err := os.Stat(path); err == nil && fi.Size() > 1<<20 {
		os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, nil)))
}
