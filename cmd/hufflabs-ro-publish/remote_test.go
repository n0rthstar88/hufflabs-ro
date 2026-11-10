package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"hufflabs-ro/internal/manifest"
)

type fakeRemote struct {
	live  []byte
	blobs map[string]bool
	calls []string
	put   map[string]string
	keep  map[string]bool
}

func (f *fakeRemote) Manifest(context.Context) ([]byte, error) {
	f.calls = append(f.calls, "Manifest")
	return f.live, nil
}
func (f *fakeRemote) HaveBlobs(context.Context) (map[string]bool, error) {
	f.calls = append(f.calls, "HaveBlobs")
	return f.blobs, nil
}
func (f *fakeRemote) PutBlobs(_ context.Context, b map[string]string) error {
	f.calls = append(f.calls, "PutBlobs")
	f.put = b
	return nil
}
func (f *fakeRemote) PutManifest(_ context.Context, b []byte) error {
	f.calls = append(f.calls, "PutManifest")
	f.live = b
	return nil
}
func (f *fakeRemote) Prune(_ context.Context, keep map[string]bool) error {
	f.calls = append(f.calls, "Prune")
	f.keep = keep
	return nil
}

func keyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return pub, priv
}

func tree(t *testing.T, files map[string]string) map[string]string {
	dir := t.TempDir()
	out := map[string]string{}
	for k, v := range files {
		mk(t, dir, k, v)
		out[k] = filepath.Join(dir, filepath.FromSlash(k))
	}
	return out
}

func newHasher(t *testing.T) *hasher { return &hasher{path: filepath.Join(t.TempDir(), "hashes.json")} }

func TestPublishOrder(t *testing.T) {
	pub, priv := keyPair(t)
	now := time.Unix(2_000_000_000, 0)
	// previous release: a.txt "v1", b.txt "b"
	r := &fakeRemote{blobs: map[string]bool{}}
	h := newHasher(t)
	if err := Publish(context.Background(), r, h, tree(t, map[string]string{"a.txt": "v1", "b.txt": "b"}), priv, now, func(string) {}); err != nil {
		t.Fatal(err)
	}
	prev, _ := manifest.Verify(r.live, []ed25519.PublicKey{pub})
	for _, e := range prev.Files {
		r.blobs[e.SHA256] = true
	}
	r.calls = nil
	if err := Publish(context.Background(), r, h, tree(t, map[string]string{"a.txt": "v2", "b.txt": "b"}), priv, now.Add(time.Hour), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.calls, ",") != "Manifest,HaveBlobs,PutBlobs,PutManifest,Prune" {
		t.Errorf("calls %v", r.calls)
	}
	if len(r.put) != 1 || r.put[shaOf("v2")] == "" {
		t.Errorf("uploaded %v, want only v2", r.put)
	}
	if !r.keep[shaOf("v1")] || !r.keep[shaOf("v2")] || !r.keep[shaOf("b")] || len(r.keep) != 3 {
		t.Errorf("keep %v, want new ∪ previous", r.keep)
	}
	m, err := manifest.Verify(r.live, []ed25519.PublicKey{pub})
	if err != nil || m.Serial != now.Add(time.Hour).Unix() {
		t.Errorf("manifest %+v %v", m, err)
	}
}

func TestPublishSerialIncreases(t *testing.T) {
	_, priv := keyPair(t)
	future := manifest.Manifest{Serial: 9_000_000_000}
	env, _ := manifest.Sign(future, priv)
	r := &fakeRemote{live: env, blobs: map[string]bool{}}
	if err := Publish(context.Background(), r, newHasher(t), tree(t, map[string]string{"a.txt": "x"}), priv, time.Unix(2_000_000_000, 0), func(string) {}); err != nil {
		t.Fatal(err)
	}
	m, _ := manifest.Verify(r.live, []ed25519.PublicKey{priv.Public().(ed25519.PublicKey)})
	if m.Serial != 9_000_000_001 {
		t.Errorf("serial %d", m.Serial)
	}
}

func TestPublishNothingChanged(t *testing.T) {
	_, priv := keyPair(t)
	r := &fakeRemote{blobs: map[string]bool{}}
	h := newHasher(t)
	tr := tree(t, map[string]string{"a.txt": "x"})
	Publish(context.Background(), r, h, tr, priv, time.Unix(2_000_000_000, 0), func(string) {})
	r.calls = nil
	var msgs []string
	if err := Publish(context.Background(), r, h, tr, priv, time.Unix(2_000_000_100, 0), func(s string) { msgs = append(msgs, s) }); err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.calls, ",") != "Manifest" || !strings.Contains(strings.Join(msgs, "\n"), "Nothing changed") {
		t.Errorf("calls %v msgs %v", r.calls, msgs)
	}
}

func TestPublishRefusesForeignLiveManifest(t *testing.T) {
	_, mine := keyPair(t)
	_, theirs := keyPair(t)
	env, _ := manifest.Sign(manifest.Manifest{Serial: 1}, theirs)
	r := &fakeRemote{live: env, blobs: map[string]bool{}}
	err := Publish(context.Background(), r, newHasher(t), tree(t, map[string]string{"a.txt": "x"}), mine, time.Now(), func(string) {})
	if err == nil || len(r.calls) != 1 {
		t.Errorf("err %v calls %v", err, r.calls)
	}
}

func TestHasherCachesBySizeAndMtime(t *testing.T) {
	h := newHasher(t)
	tr := tree(t, map[string]string{"a.txt": "aaaa"})
	s1, _ := h.sum(tr["a.txt"])
	fi, _ := os.Stat(tr["a.txt"])
	os.WriteFile(tr["a.txt"], []byte("bbbb"), 0o644)
	os.Chtimes(tr["a.txt"], time.Now(), fi.ModTime())
	if s2, _ := h.sum(tr["a.txt"]); s2 != s1 {
		t.Error("cache not used for same size+mtime")
	}
	h.save()
	h2 := &hasher{path: h.path}
	if s3, _ := h2.sum(tr["a.txt"]); s3 != s1 {
		t.Error("cache not persisted")
	}
}

func TestKeygenRefusesOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "k", "publish.key")
	pub, err := keygen(p)
	if err != nil || len(pub) == 0 {
		t.Fatal(err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
	if _, err := keygen(p); err == nil {
		t.Error("overwrote key")
	}
	if _, err := loadKey(p); err != nil {
		t.Error(err)
	}
}

// The server-side scripts run locally against a temp folder, through the same
// runner interface ssh uses.
func TestScriptsAgainstLocalFolder(t *testing.T) {
	dir := t.TempDir()
	r := &scriptRemote{run: localRun, dir: dir}
	ctx := context.Background()
	if b, err := r.Manifest(ctx); err != nil || b != nil {
		t.Fatalf("empty Manifest = %q %v", b, err)
	}
	tr := tree(t, map[string]string{"a.txt": "aaa", "b.txt": "bbb"})
	blobs := map[string]string{shaOf("aaa"): tr["a.txt"], shaOf("bbb"): tr["b.txt"]}
	if err := r.PutBlobs(ctx, blobs); err != nil {
		t.Fatal(err)
	}
	have, err := r.HaveBlobs(ctx)
	if err != nil || len(have) != 2 || !have[shaOf("aaa")] {
		t.Fatalf("HaveBlobs %v %v", have, err)
	}
	if err := r.PutManifest(ctx, []byte(`{"m":1}`)); err != nil {
		t.Fatal(err)
	}
	if b, _ := r.Manifest(ctx); !bytes.Equal(b, []byte(`{"m":1}`)) {
		t.Errorf("Manifest %q", b)
	}
	if err := r.Prune(ctx, map[string]bool{shaOf("aaa"): true}); err != nil {
		t.Fatal(err)
	}
	have, _ = r.HaveBlobs(ctx)
	var names []string
	for k := range have {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) != 1 || names[0] != shaOf("aaa") {
		t.Errorf("after prune %v", names)
	}
	// a blob whose content doesn't match its name is rejected and not kept
	bad := tree(t, map[string]string{"x": "not this"})
	if err := r.PutBlobs(ctx, map[string]string{shaOf("something else"): bad["x"]}); err == nil {
		t.Error("mismatched blob accepted")
	}
	if have, _ = r.HaveBlobs(ctx); have[shaOf("something else")] {
		t.Error("mismatched blob kept")
	}
	_ = errors.New
}

func shaOf(s string) string { return sha256hex([]byte(s)) }

// truncating cuts the script's input short, like an ssh connection dropping.
func truncating(n int64) runner {
	return func(ctx context.Context, script string, stdin io.Reader, stdout io.Writer) error {
		if stdin != nil {
			stdin = io.LimitReader(stdin, n)
		}
		return localRun(ctx, script, stdin, stdout)
	}
}

func TestPutManifestRefusesTruncatedUpload(t *testing.T) {
	dir := t.TempDir()
	good := &scriptRemote{run: localRun, dir: dir}
	if err := good.PutManifest(context.Background(), []byte(`{"old":true}`)); err != nil {
		t.Fatal(err)
	}
	cut := &scriptRemote{run: truncating(5), dir: dir}
	if err := cut.PutManifest(context.Background(), []byte(`{"new":"longer manifest"}`)); err == nil {
		t.Error("truncated manifest accepted")
	}
	if b, _ := good.Manifest(context.Background()); string(b) != `{"old":true}` {
		t.Errorf("live manifest now %q", b)
	}
}

func TestPruneRefusesTruncatedOrEmptyKeepList(t *testing.T) {
	dir := t.TempDir()
	r := &scriptRemote{run: localRun, dir: dir}
	tr := tree(t, map[string]string{"a": "aaa", "b": "bbb", "c": "ccc"})
	r.PutBlobs(context.Background(), map[string]string{shaOf("aaa"): tr["a"], shaOf("bbb"): tr["b"], shaOf("ccc"): tr["c"]})
	keep := map[string]bool{shaOf("aaa"): true, shaOf("bbb"): true, shaOf("ccc"): true}
	cut := &scriptRemote{run: truncating(70), dir: dir}
	if err := cut.Prune(context.Background(), keep); err == nil {
		t.Error("truncated keep list accepted")
	}
	if err := r.Prune(context.Background(), map[string]bool{}); err == nil {
		t.Error("empty keep list accepted")
	}
	if have, _ := r.HaveBlobs(context.Background()); len(have) != 3 {
		t.Errorf("blobs deleted: %d left", len(have))
	}
}
