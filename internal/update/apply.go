package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"hufflabs-ro/internal/manifest"
)

var (
	ErrDiskFull     = errors.New("not enough disk space")
	ErrFileInUse    = errors.New("a game file is in use")
	ErrBadDownload  = errors.New("downloaded file was damaged")
	ErrServerFailed = errors.New("update server error")
)

// Progress is reported while files download.
type Progress struct {
	FilesDone, FilesTotal int
	BytesDone, BytesTotal int64
	Current               string
}

// test hooks
var (
	rename   = os.Rename
	copyBody = func(w io.Writer, r io.Reader) (int64, error) { return io.CopyBuffer(w, r, make([]byte, 64<<10)) }
)

const workers = 4

func classify(err error) error {
	switch {
	case err == nil, errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case isDiskFull(err):
		return fmt.Errorf("%w: %v", ErrDiskFull, err)
	case isInUse(err):
		return fmt.Errorf("%w: %v", ErrFileInUse, err)
	}
	return err
}

type applier struct {
	c         *http.Client
	site, dir string
	tmp       string
	mu        sync.Mutex
	st        State
	pr        Progress
	report    func(Progress)
	self      string
}

func (a *applier) add(bytes int64, fileDone bool, current string) {
	a.mu.Lock()
	a.pr.BytesDone += bytes
	if fileDone {
		a.pr.FilesDone++
	}
	if current != "" {
		a.pr.Current = current
	}
	pr := a.pr
	a.mu.Unlock()
	if a.report != nil {
		a.report(pr)
	}
}

type countingWriter struct {
	w io.Writer
	a *applier
}

func (c countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.a.add(int64(n), false, "")
	return n, err
}

// Apply carries out p: downloads each file to .hufflabs-launcher/tmp with resume,
// checks its size and hash, renames it into place, then removes p.Delete. State is
// saved after every file, and its Serial moves to m.Serial only when all succeeded.
func Apply(ctx context.Context, c *http.Client, siteURL, dir string, p Plan, st State, m manifest.Manifest, report func(Progress)) (State, error) {
	a := &applier{c: c, site: strings.TrimRight(siteURL, "/"), dir: dir, tmp: filepath.Join(dir, StateDir, "tmp"),
		st: st.clone(), report: report, self: p.Self, pr: Progress{FilesTotal: len(p.Download), BytesTotal: p.Bytes}}
	if err := os.MkdirAll(a.tmp, 0o755); err != nil {
		return st, classify(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Files with the same content share one blob: fetch it once per group.
	var groups [][]manifest.Entry
	idx := map[string]int{}
	for _, e := range p.Download {
		if i, ok := idx[e.SHA256]; ok {
			groups[i] = append(groups[i], e)
			continue
		}
		idx[e.SHA256] = len(groups)
		groups = append(groups, []manifest.Entry{e})
	}
	jobs := make(chan []manifest.Entry)
	errs := make(chan error, len(p.Download))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for g := range jobs {
				if err := a.group(ctx, g); err != nil {
					errs <- err
					cancel()
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, g := range groups {
			select {
			case jobs <- g:
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	var first error
	for err := range errs {
		if first == nil || errors.Is(first, context.Canceled) {
			first = err
		}
	}
	if first == nil && ctx.Err() != nil {
		first = ctx.Err()
	}
	if first != nil {
		return a.st, classify(first)
	}
	for _, path := range p.Delete {
		if manifest.Unmanaged(path) || manifest.ValidPath(path) != nil {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return a.st, classify(err)
		}
		delete(a.st.Files, path)
	}
	a.st.Serial = m.Serial
	if err := a.st.Save(dir); err != nil {
		return a.st, classify(err)
	}
	return a.st, nil
}

// group downloads one blob and puts it at every path in g (same content).
func (a *applier) group(ctx context.Context, g []manifest.Entry) error {
	part := filepath.Join(a.tmp, g[0].SHA256+".part")
	if err := a.fetch(ctx, g[0], part); err != nil {
		return err
	}
	for i, e := range g {
		src := part
		if i < len(g)-1 {
			src = filepath.Join(a.tmp, e.SHA256+"."+strconv.Itoa(i)+".copy")
			if err := copyFile(part, src); err != nil {
				return err
			}
			a.add(e.Size, false, "") // counted once by the download
		}
		if err := a.place(e, src); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := copyBody(out, in); err != nil {
		out.Close()
		os.Remove(to)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fetch fills part with e's blob, checked against its hash.
func (a *applier) fetch(ctx context.Context, e manifest.Entry, part string) error {
	mismatches, failures := 0, 0
	for {
		err := a.download(ctx, e, part)
		if err == nil {
			sum, herr := hashFile(part)
			if herr != nil {
				return herr
			}
			if sum == e.SHA256 {
				break
			}
			os.Remove(part)
			a.add(-e.Size, false, "")
			if mismatches++; mismatches >= 2 {
				return fmt.Errorf("%w: %s", ErrBadDownload, e.Path)
			}
			continue
		}
		if ctx.Err() != nil || isDiskFull(err) {
			return err
		}
		if failures++; failures >= 3 {
			return err
		}
	}
	return nil
}

// place renames src over e's path and records it.
func (a *applier) place(e manifest.Entry, src string) error {
	target := filepath.Join(a.dir, filepath.FromSlash(e.Path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	swap := rename
	if a.self != "" && strings.EqualFold(e.Path, a.self) {
		swap = func(src, dst string) error { return SwapSelf(dst, src) }
	}
	if err := swap(src, target); err != nil {
		return err
	}
	fi, err := os.Stat(target)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.st.Files[e.Path] = FileState{Size: fi.Size(), ModUnix: fi.ModTime().UnixNano(), SHA256: e.SHA256}
	err = a.st.Save(a.dir)
	a.mu.Unlock()
	a.add(0, true, e.Path)
	return err
}

// download fills part up to e.Size, resuming from what is already there.
func (a *applier) download(ctx context.Context, e manifest.Entry, part string) error {
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	have := fi.Size()
	if have > e.Size {
		if err := f.Truncate(0); err != nil {
			return err
		}
		have = 0
	}
	a.add(have, false, e.Path)
	if have == e.Size {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.site+"/update/files/"+e.SHA256, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	resp, err := a.c.Do(req)
	if err != nil {
		a.add(-have, false, "")
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
	case resp.StatusCode == http.StatusOK:
		if err := f.Truncate(0); err != nil {
			return err
		}
		a.add(-have, false, "")
		have = 0
	default:
		a.add(-have, false, "")
		return fmt.Errorf("%w: HTTP %d for %s", ErrServerFailed, resp.StatusCode, e.Path)
	}
	if _, err := f.Seek(have, io.SeekStart); err != nil {
		return err
	}
	n, err := copyBody(countingWriter{f, a}, io.LimitReader(resp.Body, e.Size-have))
	if err != nil {
		a.add(-(have + n), false, "")
		return err
	}
	if have+n != e.Size {
		a.add(-(have + n), false, "")
		return fmt.Errorf("short download of %s", e.Path)
	}
	return f.Sync()
}
