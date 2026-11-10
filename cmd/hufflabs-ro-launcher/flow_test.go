package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"hufflabs-ro/internal/manifest"
	"hufflabs-ro/internal/update"
)

type fake struct {
	mu        sync.Mutex
	views     []View
	fetchErrs []error // consumed one per Fetch
	m         manifest.Manifest
	st        update.State
	plan      update.Plan
	applyErrs []error
	lockErr   error
	started   int
	applied   []update.Plan
	progress  []update.Progress
}

func (f *fake) deps() Deps {
	return Deps{
		Configured: true,
		Installed:  true,
		SelfRel:    "HuffLabsRO-Launcher.exe",
		Lock: func() (func(), error) {
			if f.lockErr != nil {
				return nil, f.lockErr
			}
			return func() {}, nil
		},
		CleanupOld: func() {},
		Fetch: func(context.Context) (manifest.Manifest, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.fetchErrs) > 0 {
				err := f.fetchErrs[0]
				f.fetchErrs = f.fetchErrs[1:]
				if err != nil {
					return manifest.Manifest{}, err
				}
			}
			return f.m, nil
		},
		LoadState: func() (update.State, error) { return f.st, nil },
		MakePlan: func(st update.State, m manifest.Manifest, progress func(int, int)) (update.Plan, update.State, error) {
			progress(1, 1)
			return f.plan, st, nil
		},
		Apply: func(ctx context.Context, p update.Plan, st update.State, m manifest.Manifest, report func(update.Progress)) (update.State, error) {
			f.mu.Lock()
			f.applied = append(f.applied, p)
			var err error
			if len(f.applyErrs) > 0 {
				err, f.applyErrs = f.applyErrs[0], f.applyErrs[1:]
			}
			prs := f.progress
			f.mu.Unlock()
			for _, pr := range prs {
				report(pr)
			}
			st.Serial = m.Serial
			return st, err
		},
		StartGame: func() error { f.mu.Lock(); f.started++; f.mu.Unlock(); return nil },
	}
}

func (f *fake) show(v View) { f.mu.Lock(); f.views = append(f.views, v); f.mu.Unlock() }

func (f *fake) last() View {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.views) == 0 {
		return View{}
	}
	return f.views[len(f.views)-1]
}

// run starts Run and answers each waiting view with the next choice.
func (f *fake) run(t *testing.T, choices ...Choice) Result {
	t.Helper()
	ch := make(chan Choice)
	done := make(chan Result, 1)
	go func() { done <- Run(context.Background(), f.deps(), f.show, ch) }()
	for _, c := range choices {
		select {
		case ch <- c:
		case r := <-done:
			t.Fatalf("Run returned %+v before choice %v; views %+v", r, c, f.views)
		case <-time.After(2 * time.Second):
			t.Fatalf("Run not waiting for a choice; views %+v", f.views)
		}
	}
	select {
	case r := <-done:
		return r
	case <-time.After(2 * time.Second):
		t.Fatalf("Run didn't finish; last view %+v", f.last())
	}
	return Result{}
}

func phases(vs []View) []Phase {
	var out []Phase
	for _, v := range vs {
		if len(out) == 0 || out[len(out)-1] != v.Phase {
			out = append(out, v.Phase)
		}
	}
	return out
}

func TestFlowUpToDateStartsGame(t *testing.T) {
	f := &fake{m: manifest.Manifest{Serial: 3}, st: update.State{Serial: 3}}
	r := f.run(t)
	if !r.Started || f.started != 1 {
		t.Errorf("result %+v, started %d", r, f.started)
	}
	if got := phases(f.views); fmt.Sprint(got) != fmt.Sprint([]Phase{Checking, Scanning, Starting}) {
		t.Errorf("phases %v", got)
	}
	if len(f.applied) != 0 {
		t.Error("Apply called with nothing to do")
	}
}

func TestFlowUnreachableOffersRetryAndPlay(t *testing.T) {
	f := &fake{fetchErrs: []error{update.ErrUnavailable, update.ErrUnavailable}, m: manifest.Manifest{Serial: 1}, st: update.State{Serial: 1}}
	ch := make(chan Choice)
	done := make(chan Result, 1)
	go func() { done <- Run(context.Background(), f.deps(), f.show, ch) }()
	ch <- Retry
	time.Sleep(50 * time.Millisecond)
	v := f.last()
	if v.Phase != Offline || v.Title != "Can't reach the HuffLabs server" || v.Detail != "Connect Tailscale, then press Retry." || !v.CanRetry || !v.CanPlay {
		t.Errorf("offline view %+v", v)
	}
	ch <- PlayAnyway
	r := <-done
	if !r.Started || f.started != 1 || len(f.applied) != 0 {
		t.Errorf("result %+v started %d", r, f.started)
	}
}

func TestFlowRetryThenUpdates(t *testing.T) {
	f := &fake{fetchErrs: []error{update.ErrUnavailable}, m: manifest.Manifest{Serial: 2}, st: update.State{Serial: 1},
		plan: update.Plan{Download: []manifest.Entry{{Path: "a.txt", Size: 1}}, Bytes: 1}}
	r := f.run(t, Retry)
	if !r.Started || len(f.applied) != 1 {
		t.Errorf("result %+v applied %d", r, len(f.applied))
	}
}

func TestFlowRollbackRefuses(t *testing.T) {
	f := &fake{m: manifest.Manifest{Serial: 1}, st: update.State{Serial: 5}}
	ch := make(chan Choice)
	done := make(chan Result, 1)
	go func() { done <- Run(context.Background(), f.deps(), f.show, ch) }()
	time.Sleep(50 * time.Millisecond)
	if v := f.last(); v.Phase != Rollback || v.Detail != "The server sent an older update than you have." || !v.CanPlay {
		t.Errorf("view %+v", v)
	}
	ch <- PlayAnyway
	<-done
	if len(f.applied) != 0 || f.started != 1 {
		t.Errorf("applied %d started %d", len(f.applied), f.started)
	}
}

func TestFlowApplyErrorsArePlain(t *testing.T) {
	for err, want := range map[error]string{
		update.ErrFileInUse:    "Close HuffLabs RO, then press Retry.",
		update.ErrDiskFull:     "Not enough disk space. Free some space, then press Retry.",
		errors.New("tls: bad"): "The update didn't finish. Press Retry to continue where it stopped.",
		update.ErrBadDownload:  "The update didn't finish. Press Retry to continue where it stopped.",
	} {
		f := &fake{m: manifest.Manifest{Serial: 2}, st: update.State{Serial: 1}, applyErrs: []error{fmt.Errorf("x: %w", err)},
			plan: update.Plan{Download: []manifest.Entry{{Path: "a.txt"}}}}
		ch := make(chan Choice)
		done := make(chan Result, 1)
		go func() { done <- Run(context.Background(), f.deps(), f.show, ch) }()
		time.Sleep(50 * time.Millisecond)
		if v := f.last(); v.Phase != Failed || v.Detail != want || !v.CanRetry || !v.CanPlay {
			t.Errorf("%v: view %+v", err, v)
		}
		ch <- Retry // second Apply succeeds
		<-done
		if len(f.applied) != 2 || f.started != 1 {
			t.Errorf("%v: applied %d started %d", err, len(f.applied), f.started)
		}
	}
}

func TestFlowSelfUpdateReexecs(t *testing.T) {
	f := &fake{m: manifest.Manifest{Serial: 2}, st: update.State{Serial: 1},
		plan: update.Plan{Download: []manifest.Entry{{Path: "data/x.bmp"}, {Path: "HuffLabsRO-Launcher.exe"}}}}
	r := f.run(t)
	if !r.Reexec || r.Started || f.started != 0 {
		t.Errorf("result %+v started %d", r, f.started)
	}
	if len(f.applied) != 1 || f.applied[0].Self != "HuffLabsRO-Launcher.exe" {
		t.Errorf("applied %+v", f.applied)
	}
}

func TestFlowSecondInstanceQuits(t *testing.T) {
	f := &fake{lockErr: update.ErrAlreadyRunning}
	r := f.run(t)
	if r.Started || len(f.views) != 0 {
		t.Errorf("result %+v views %+v", r, f.views)
	}
}

func TestFlowUnconfigured(t *testing.T) {
	f := &fake{}
	d := f.deps()
	d.Configured = false
	ch := make(chan Choice)
	done := make(chan Result, 1)
	go func() { done <- Run(context.Background(), d, f.show, ch) }()
	time.Sleep(50 * time.Millisecond)
	if v := f.last(); v.Title != "This launcher was built without a server address." || v.CanRetry || !v.CanPlay {
		t.Errorf("view %+v", v)
	}
	ch <- PlayAnyway
	<-done
}

func TestFlowProgressText(t *testing.T) {
	got := downloadText(update.Progress{FilesDone: 2, FilesTotal: 12, BytesDone: 41_000_000, BytesTotal: 120_000_000})
	if got != "Downloading 3 of 12 files — 41 MB of 120 MB" {
		t.Errorf("got %q", got)
	}
	if got := downloadText(update.Progress{FilesDone: 1, FilesTotal: 1, BytesDone: 3_845_198_853, BytesTotal: 3_845_198_853}); got != "Downloading 1 of 1 files — 3.8 GB of 3.8 GB" {
		t.Errorf("got %q", got)
	}
}

// Run from a folder that isn't a HuffLabs RO install (say the Desktop), the
// launcher must not pull 4.5 GB of game files into it.
func TestFlowRefusesOutsideInstallFolder(t *testing.T) {
	f := &fake{}
	d := f.deps()
	d.Installed = false
	fetched := false
	d.Fetch = func(context.Context) (manifest.Manifest, error) { fetched = true; return manifest.Manifest{}, nil }
	locked := false
	d.Lock = func() (func(), error) { locked = true; return func() {}, nil }
	ch := make(chan Choice)
	done := make(chan Result, 1)
	go func() { done <- Run(context.Background(), d, f.show, ch) }()
	time.Sleep(50 * time.Millisecond)
	v := f.last()
	if v.Title != "Run HuffLabs RO from its install folder" || v.Detail != "Use the HuffLabs RO shortcut on your desktop or in the Start menu." || v.CanRetry || v.CanPlay {
		t.Errorf("view %+v", v)
	}
	ch <- Quit
	if r := <-done; r.Started || fetched || locked {
		t.Errorf("result %+v fetched %v locked %v", r, fetched, locked)
	}
}
