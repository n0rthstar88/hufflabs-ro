package update

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"hufflabs-ro/internal/manifest"
)

// blobServer serves /update/files/<sha> with Range support and records requests.
type blobServer struct {
	*httptest.Server
	mu     sync.Mutex
	blobs  map[string][]byte
	ranges []string
	hits   map[string]int
	bad    map[string]bool // serve wrong bytes for these
	block  chan struct{}   // if set, stall after the first half
}

func newBlobServer(t *testing.T) *blobServer {
	b := &blobServer{blobs: map[string][]byte{}, hits: map[string]int{}, bad: map[string]bool{}}
	b.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sha := strings.TrimPrefix(r.URL.Path, "/update/files/")
		b.mu.Lock()
		data, ok := b.blobs[sha]
		b.hits[sha]++
		b.ranges = append(b.ranges, r.Header.Get("Range"))
		bad, block := b.bad[sha], b.block
		b.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		if bad {
			data = []byte(strings.Repeat("x", len(data)))
		}
		start := 0
		if rg := r.Header.Get("Range"); strings.HasPrefix(rg, "bytes=") {
			start, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(len(data)-1)+"/"+strconv.Itoa(len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		}
		if block != nil {
			w.Write(data[start : start+(len(data)-start)/2])
			w.(http.Flusher).Flush()
			<-block
			return
		}
		w.Write(data[start:])
	}))
	t.Cleanup(b.Close)
	return b
}

func (b *blobServer) add(content []byte) manifest.Entry {
	e := entry("x", content)
	b.mu.Lock()
	b.blobs[e.SHA256] = content
	b.mu.Unlock()
	return e
}

func setup(t *testing.T, files map[string]string) (string, *blobServer, manifest.Manifest) {
	t.Helper()
	dir := t.TempDir()
	srv := newBlobServer(t)
	m := manifest.Manifest{Serial: 10}
	for p, c := range files {
		e := srv.add([]byte(c))
		e.Path = p
		m.Files = append(m.Files, e)
	}
	return dir, srv, m
}

func plan(t *testing.T, dir string, m manifest.Manifest) (Plan, State) {
	t.Helper()
	st, _ := LoadState(dir)
	p, st, err := MakePlan(dir, st, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p, st
}

func TestApplyDownloadsAndSwaps(t *testing.T) {
	dir, srv, m := setup(t, map[string]string{"a.txt": "new a", "data/sub/b.txt": "new b"})
	writeFile(t, dir, "a.txt", []byte("old a"))
	p, st := plan(t, dir, m)
	var last Progress
	st, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, func(pr Progress) { last = pr })
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "a.txt") != "new a" || readFile(t, dir, "data/sub/b.txt") != "new b" {
		t.Error("files not replaced")
	}
	if st.Serial != 10 || last.FilesDone != 2 || last.FilesTotal != 2 || last.BytesDone != 10 {
		t.Errorf("serial %d, progress %+v", st.Serial, last)
	}
	if saved, _ := LoadState(dir); saved.Serial != 10 || saved.Files["a.txt"].SHA256 != shaHex([]byte("new a")) {
		t.Errorf("saved state %+v", saved)
	}
	left, _ := os.ReadDir(filepath.Join(dir, StateDir, "tmp"))
	if len(left) != 0 {
		t.Errorf("tmp not empty: %v", left)
	}
	// a second plan finds nothing to do
	if p2, _ := plan(t, dir, m); len(p2.Download) != 0 {
		t.Errorf("second plan %v", p2.Download)
	}
}

func TestApplyResumesPartial(t *testing.T) {
	content := strings.Repeat("0123456789", 100)
	dir, srv, m := setup(t, map[string]string{"big.bin": content})
	part := filepath.Join(dir, StateDir, "tmp", m.Files[0].SHA256+".part")
	os.MkdirAll(filepath.Dir(part), 0o755)
	os.WriteFile(part, []byte(content[:500]), 0o644)
	p, st := plan(t, dir, m)
	if _, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, nil); err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "big.bin") != content {
		t.Error("resumed file wrong")
	}
	if len(srv.ranges) != 1 || srv.ranges[0] != "bytes=500-" {
		t.Errorf("ranges = %v, want [bytes=500-]", srv.ranges)
	}
}

func TestApplyBadBlobKeepsOldFile(t *testing.T) {
	dir, srv, m := setup(t, map[string]string{"a.txt": "new a"})
	writeFile(t, dir, "a.txt", []byte("old"))
	srv.bad[m.Files[0].SHA256] = true
	p, st := plan(t, dir, m)
	st2, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, nil)
	if err == nil || readFile(t, dir, "a.txt") != "old" || st2.Serial == 10 {
		t.Errorf("err %v, file %q, serial %d", err, readFile(t, dir, "a.txt"), st2.Serial)
	}
	if srv.hits[m.Files[0].SHA256] != 2 {
		t.Errorf("tried %d times, want 2", srv.hits[m.Files[0].SHA256])
	}
}

func TestApplyFileInUse(t *testing.T) {
	dir, srv, m := setup(t, map[string]string{"HuffLabsRO.exe": "new exe"})
	writeFile(t, dir, "HuffLabsRO.exe", []byte("old exe"))
	old := rename
	rename = func(a, b string) error { return &os.LinkError{Op: "rename", Old: a, New: b, Err: inUseErrno} }
	defer func() { rename = old }()
	p, st := plan(t, dir, m)
	_, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, nil)
	if !errors.Is(err, ErrFileInUse) {
		t.Errorf("err = %v, want ErrFileInUse", err)
	}
	if readFile(t, dir, "HuffLabsRO.exe") != "old exe" {
		t.Error("in-use file changed")
	}
}

func TestApplyDiskFull(t *testing.T) {
	dir, srv, m := setup(t, map[string]string{"a.txt": "new a"})
	writeFile(t, dir, "a.txt", []byte("old"))
	old := copyBody
	copyBody = func(w io.Writer, r io.Reader) (int64, error) {
		return 0, &os.PathError{Op: "write", Path: "x", Err: diskFullErrno}
	}
	defer func() { copyBody = old }()
	p, st := plan(t, dir, m)
	_, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, nil)
	if !errors.Is(err, ErrDiskFull) {
		t.Errorf("err = %v, want ErrDiskFull", err)
	}
	if readFile(t, dir, "a.txt") != "old" {
		t.Error("file changed on disk full")
	}
}

func TestApplyDeletes(t *testing.T) {
	dir, srv, _ := setup(t, nil)
	writeFile(t, dir, "gone.txt", []byte("x"))
	writeFile(t, dir, "savedata/keep.lua", []byte("x"))
	st := State{Serial: 1, Files: map[string]FileState{"gone.txt": {}, "savedata/keep.lua": {}}}
	p := Plan{Delete: []string{"gone.txt", "savedata/keep.lua", "../outside.txt"}}
	st, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, manifest.Manifest{Serial: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "gone.txt") != "<missing>" || readFile(t, dir, "savedata/keep.lua") != "x" {
		t.Error("delete wrong")
	}
	if _, ok := st.Files["gone.txt"]; ok {
		t.Error("deleted file still in state")
	}
}

func TestApplyCancel(t *testing.T) {
	content := strings.Repeat("z", 1<<20)
	dir, srv, m := setup(t, map[string]string{"big.bin": content})
	srv.block = make(chan struct{})
	defer close(srv.block)
	ctx, cancel := context.WithCancel(context.Background())
	p, st := plan(t, dir, m)
	done := make(chan error, 1)
	go func() {
		_, err := Apply(ctx, srv.Client(), srv.URL, dir, p, st, m, func(pr Progress) {
			if pr.BytesDone > 0 {
				cancel()
			}
		})
		done <- err
	}()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	fi, err := os.Stat(filepath.Join(dir, StateDir, "tmp", m.Files[0].SHA256+".part"))
	if err != nil || fi.Size() == 0 {
		t.Errorf("partial not kept: %v", err)
	}
}

// The real client has hundreds of identical files. They share one blob and must
// download it once without workers tripping over each other's temp file.
func TestApplySameContentManyPaths(t *testing.T) {
	content := strings.Repeat("s", 4<<20)
	files := map[string]string{}
	for i := 0; i < 8; i++ {
		files["data/sprite/"+strconv.Itoa(i)+".spr"] = content
	}
	for run := 0; run < 5; run++ {
		dir, srv, m := setup(t, files)
		p, st := plan(t, dir, m)
		var last Progress
		if _, err := Apply(context.Background(), srv.Client(), srv.URL, dir, p, st, m, func(pr Progress) { last = pr }); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		for path := range files {
			if readFile(t, dir, path) != content {
				t.Fatalf("run %d: %s wrong", run, path)
			}
		}
		if n := srv.hits[m.Files[0].SHA256]; n != 1 {
			t.Errorf("run %d: blob fetched %d times, want 1", run, n)
		}
		if last.FilesDone != 8 || last.BytesDone != last.BytesTotal {
			t.Errorf("run %d: progress %+v", run, last)
		}
	}
}
