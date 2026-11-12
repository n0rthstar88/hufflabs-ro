package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"hufflabs-ro/internal/manifest"
	"hufflabs-ro/internal/update"
)

type Phase int

const (
	Checking Phase = iota
	Scanning
	Downloading
	Starting
	Offline
	Failed
	Rollback
)

// View is everything the window shows. Fraction < 0 means "busy, no figure".
type View struct {
	Phase             Phase
	Title, Detail     string
	Fraction          float64
	CanRetry, CanPlay bool
}

type Choice int

const (
	Retry Choice = iota + 1
	PlayAnyway
	Quit
)

// Deps is what the flow needs from the outside world, so tests can fake it.
type Deps struct {
	Configured bool   // built with a server address and keys
	Installed  bool   // the exe sits in a HuffLabs RO install folder
	SelfRel    string // the launcher's own path in the manifest
	Lock       func() (func(), error)
	CleanupOld func()
	Fetch      func(context.Context) (manifest.Manifest, error)
	LoadState  func() (update.State, error)
	MakePlan   func(update.State, manifest.Manifest, func(int, int)) (update.Plan, update.State, error)
	Apply      func(context.Context, update.Plan, update.State, manifest.Manifest, func(update.Progress)) (update.State, error)
	StartGame  func() error
}

type Result struct {
	Started bool // the game was started
	Reexec  bool // the launcher replaced itself; start the new one
}

const retryText = "The update didn't finish. Press Retry to continue where it stopped."

// Run checks for updates, applies them and starts the game, asking the player
// through choices whenever something needs a decision.
func Run(ctx context.Context, d Deps, show func(View), choices <-chan Choice) Result {
	if !d.Installed {
		show(View{Phase: Failed, Title: "Run HuffLabs RO from its install folder", Detail: "Use the HuffLabs RO shortcut on your desktop or in the Start menu.", Fraction: -1})
		select {
		case <-choices:
		case <-ctx.Done():
		}
		return Result{}
	}
	release, err := d.Lock()
	if err != nil {
		slog.Info("not starting", "err", err)
		return Result{}
	}
	defer func() { release(); slog.Info("lock released") }()
	d.CleanupOld()

	wait := func(v View) Choice {
		show(v)
		select {
		case c := <-choices:
			return c
		case <-ctx.Done():
			return Quit
		}
	}
	// start returns true when the player pressed Retry after a failed start:
	// the caller then checks the files again, so a game exe that antivirus
	// removed is downloaded again instead of failing the same way forever.
	start := func() (Result, bool) {
		show(View{Phase: Starting, Title: "Starting HuffLabs RO…", Fraction: 1})
		err := d.StartGame()
		if err == nil {
			return Result{Started: true}, false
		}
		slog.Error("start game", "err", err)
		if wait(View{Phase: Failed, Title: "Couldn't start the game", Detail: startDetail(err), Fraction: -1, CanRetry: true}) != Retry {
			return Result{}, false
		}
		return Result{}, true
	}
	decide := func(v View) (Result, bool) { // true: go round again
		switch wait(v) {
		case Retry:
			return Result{}, true
		case PlayAnyway:
			return start()
		}
		return Result{}, false
	}

	if !d.Configured {
		for {
			r, again := decide(View{Phase: Failed, Title: "This launcher was built without a server address.", Detail: "You can still play, but it can't update.", Fraction: -1, CanPlay: true})
			if !again {
				return r
			}
		}
	}
	for {
		show(View{Phase: Checking, Title: "Checking for updates…", Fraction: -1})
		m, err := d.Fetch(ctx)
		if err != nil {
			slog.Warn("fetch manifest", "err", err)
			if r, again := decide(View{Phase: Offline, Title: "Can't reach the HuffLabs server", Detail: "Connect Tailscale, then press Retry.", Fraction: -1, CanRetry: true, CanPlay: true}); !again {
				return r
			}
			continue
		}
		st, err := d.LoadState()
		if err != nil {
			slog.Error("load state", "err", err)
		}
		if err := update.CheckSerial(st, m); err != nil {
			slog.Error("rollback", "err", err)
			if r, again := decide(View{Phase: Rollback, Title: "Update check failed", Detail: "The server sent an older update than you have.", Fraction: -1, CanRetry: true, CanPlay: true}); !again {
				return r
			}
			continue
		}
		show(View{Phase: Scanning, Title: "Checking game files…", Fraction: 0})
		p, st, err := d.MakePlan(st, m, func(done, total int) {
			if total > 0 {
				show(View{Phase: Scanning, Title: "Checking game files…", Detail: fmt.Sprintf("%d of %d files", done, total), Fraction: float64(done) / float64(total)})
			}
		})
		if err != nil {
			slog.Error("plan", "err", err)
			if r, again := decide(View{Phase: Failed, Title: "Couldn't check the game files", Detail: retryText, Fraction: -1, CanRetry: true, CanPlay: true}); !again {
				return r
			}
			continue
		}
		if len(p.Download) == 0 && len(p.Delete) == 0 && st.Serial == m.Serial {
			if r, again := start(); !again {
				return r
			}
			continue
		}
		for _, e := range p.Download {
			if d.SelfRel != "" && strings.EqualFold(e.Path, d.SelfRel) {
				p.Self = e.Path
			}
		}
		show(View{Phase: Downloading, Title: "Updating HuffLabs RO", Detail: downloadText(update.Progress{FilesTotal: len(p.Download), BytesTotal: p.Bytes}), Fraction: 0})
		_, err = d.Apply(ctx, p, st, m, func(pr update.Progress) {
			f := 1.0
			if pr.BytesTotal > 0 {
				f = float64(pr.BytesDone) / float64(pr.BytesTotal)
			}
			show(View{Phase: Downloading, Title: "Updating HuffLabs RO", Detail: downloadText(pr), Fraction: f})
		})
		if err != nil {
			slog.Error("apply", "err", err)
			detail := retryText
			switch {
			case errors.Is(err, update.ErrFileInUse):
				detail = "Close HuffLabs RO, then press Retry."
			case errors.Is(err, update.ErrDiskFull):
				detail = "Not enough disk space. Free some space, then press Retry."
			}
			if r, again := decide(View{Phase: Failed, Title: "The update didn't finish", Detail: detail, Fraction: -1, CanRetry: true, CanPlay: true}); !again {
				return r
			}
			continue
		}
		if p.Self != "" {
			return Result{Reexec: true}
		}
		if r, again := start(); !again {
			return r
		}
	}
}

func sizeText(b int64) string {
	if b >= 1_000_000_000 {
		return fmt.Sprintf("%.1f GB", float64(b)/1e9)
	}
	return fmt.Sprintf("%d MB", (b+500_000)/1_000_000)
}

func downloadText(pr update.Progress) string {
	n := pr.FilesDone + 1
	if n > pr.FilesTotal {
		n = pr.FilesTotal
	}
	return fmt.Sprintf("Downloading %d of %d files — %s of %s", n, pr.FilesTotal, sizeText(pr.BytesDone), sizeText(pr.BytesTotal))
}
