package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hufflabs-ro/internal/manifest"
)

// Remote is the server's update folder (manifest.json and files/<sha256>).
type Remote interface {
	Manifest(ctx context.Context) ([]byte, error) // nil if there is none yet
	HaveBlobs(ctx context.Context) (map[string]bool, error)
	PutBlobs(ctx context.Context, blobs map[string]string) error // sha256 -> source file
	PutManifest(ctx context.Context, b []byte) error
	Prune(ctx context.Context, keep map[string]bool) error
}

// runner runs a bash script where the update folder lives.
type runner func(ctx context.Context, script string, stdin io.Reader, stdout io.Writer) error

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func localRun(ctx context.Context, script string, stdin io.Reader, stdout io.Writer) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Stdin, cmd.Stdout = stdin, stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// sshRun runs the script inside the container through Proxmox, the way the
// repo's deploy scripts do.
func sshRun(proxmox, ct string) runner {
	return func(ctx context.Context, script string, stdin io.Reader, stdout io.Writer) error {
		cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", proxmox, "pct exec "+shq(ct)+" -- bash -c "+shq(script))
		cmd.Stdin, cmd.Stdout = stdin, stdout
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil
	}
}

type scriptRemote struct {
	run runner
	dir string
}

const setGroup = `chgrp portal "$1" 2>/dev/null || true` // the portal reads as group portal on the server

func (r *scriptRemote) Manifest(ctx context.Context) ([]byte, error) {
	var out bytes.Buffer
	if err := r.run(ctx, `f=`+shq(r.dir+"/manifest.json")+`; if [ -f "$f" ]; then cat "$f"; fi`, nil, &out); err != nil {
		return nil, err
	}
	if out.Len() == 0 {
		return nil, nil
	}
	return out.Bytes(), nil
}

func (r *scriptRemote) HaveBlobs(ctx context.Context) (map[string]bool, error) {
	var out bytes.Buffer
	if err := r.run(ctx, `ls -1 `+shq(r.dir+"/files")+` 2>/dev/null || true`, nil, &out); err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, l := range strings.Split(out.String(), "\n") {
		if isSHA(l) {
			have[l] = true
		}
	}
	return have, nil
}

func isSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// PutBlobs streams the files as one tar into files/.incoming, where each is
// checked against its name before it moves into files/.
func (r *scriptRemote) PutBlobs(ctx context.Context, blobs map[string]string) error {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		err := func() error {
			for sha, src := range blobs {
				f, err := os.Open(src)
				if err != nil {
					return err
				}
				fi, err := f.Stat()
				if err == nil {
					err = tw.WriteHeader(&tar.Header{Name: sha, Mode: 0o644, Size: fi.Size(), ModTime: time.Now(), Format: tar.FormatPAX})
				}
				if err == nil {
					_, err = io.Copy(tw, f)
				}
				f.Close()
				if err != nil {
					return err
				}
			}
			return tw.Close()
		}()
		pw.CloseWithError(err)
	}()
	script := `set -e
D=` + shq(r.dir) + `/files
mkdir -p "$D/.incoming"
rm -f "$D"/.incoming/*
tar -xf - -C "$D/.incoming"
cd "$D/.incoming"
bad=
for f in *; do
	[ -e "$f" ] || continue
	s=$(sha256sum "$f" | cut -d' ' -f1)
	if [ "$s" = "$f" ]; then
		chmod 644 "$f"; set -- "$f"; ` + setGroup + `
		mv -f "$f" "../$f"
	else
		rm -f "$f"; bad="$bad $f"
	fi
done
[ -z "$bad" ] || { echo "damaged on upload:$bad" >&2; exit 1; }`
	return r.run(ctx, script, pr, io.Discard)
}

// PutManifest only switches when the uploaded bytes hash to what was sent, so a
// dropped connection can't put a cut-off manifest live.
func (r *scriptRemote) PutManifest(ctx context.Context, b []byte) error {
	script := `set -e
D=` + shq(r.dir) + `
mkdir -p "$D/files"
cat > "$D/manifest.json.new"
if [ "$(sha256sum "$D/manifest.json.new" | cut -d' ' -f1)" != ` + shq(sha256hex(b)) + ` ]; then
	rm -f "$D/manifest.json.new"; echo "manifest upload was cut off" >&2; exit 1
fi
chmod 644 "$D/manifest.json.new"; set -- "$D/manifest.json.new"; ` + setGroup + `
mv -f "$D/manifest.json.new" "$D/manifest.json"`
	return r.run(ctx, script, bytes.NewReader(b), io.Discard)
}

// Prune deletes blobs not in keep. It refuses an empty list, and the server
// checks it received the whole list before deleting anything.
func (r *scriptRemote) Prune(ctx context.Context, keep map[string]bool) error {
	if len(keep) == 0 {
		return errors.New("refusing to prune with an empty keep list")
	}
	var list bytes.Buffer
	for k := range keep {
		list.WriteString(k + "\n")
	}
	script := `set -e
k=$(mktemp); trap 'rm -f "$k"' EXIT
cat > "$k"
if [ "$(sha256sum "$k" | cut -d' ' -f1)" != ` + shq(sha256hex(list.Bytes())) + ` ]; then
	echo "keep list was cut off; nothing deleted" >&2; exit 1
fi
cd ` + shq(r.dir+"/files") + `
for f in *; do
	[ -f "$f" ] || continue
	grep -qxF "$f" "$k" || rm -f "$f"
done`
	return r.run(ctx, script, &list, io.Discard)
}

var forceKeyChange bool

// Publish uploads the files the server doesn't have, then switches the live
// manifest to tree, then drops files neither the new nor the previous manifest
// uses. Launchers never see a manifest whose files aren't there yet.
func Publish(ctx context.Context, r Remote, h *hasher, tree map[string]string, key ed25519.PrivateKey, now time.Time, report func(string)) error {
	pub := key.Public().(ed25519.PublicKey)
	liveB, err := r.Manifest(ctx)
	if err != nil {
		return fmt.Errorf("reading the live manifest: %w", err)
	}
	var live manifest.Manifest
	if liveB != nil {
		live, err = manifest.Verify(liveB, []ed25519.PublicKey{pub})
		if err != nil && !forceKeyChange {
			return fmt.Errorf("the live manifest isn't signed by this key (%v); pass --force-key-change if you really changed keys", err)
		}
	}
	var files []manifest.Entry
	src := map[string]string{}
	paths := make([]string, 0, len(tree))
	for p := range tree {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for i, p := range paths {
		sum, err := h.sum(tree[p])
		if err != nil {
			return err
		}
		fi, err := os.Stat(tree[p])
		if err != nil {
			return err
		}
		files = append(files, manifest.Entry{Path: p, Size: fi.Size(), SHA256: sum})
		src[sum] = tree[p]
		if (i+1)%500 == 0 {
			report(fmt.Sprintf("hashed %d of %d files", i+1, len(paths)))
		}
	}
	if err := h.save(); err != nil {
		report("warning: couldn't save the hash cache: " + err.Error())
	}
	if sameFiles(live.Files, files) {
		report("Nothing changed; the server already has this.")
		return nil
	}
	have, err := r.HaveBlobs(ctx)
	if err != nil {
		return err
	}
	need := map[string]string{}
	var bytesUp int64
	for _, e := range files {
		if !have[e.SHA256] && need[e.SHA256] == "" {
			need[e.SHA256] = src[e.SHA256]
			bytesUp += e.Size
		}
	}
	if len(need) > 0 {
		report(fmt.Sprintf("uploading %d files (%.1f MB)", len(need), float64(bytesUp)/1e6))
		if err := r.PutBlobs(ctx, need); err != nil {
			return fmt.Errorf("upload: %w", err)
		}
	}
	serial := now.Unix()
	if live.Serial >= serial {
		serial = live.Serial + 1
	}
	env, err := manifest.Sign(manifest.Manifest{Serial: serial, Created: now.UTC(), Files: files}, key)
	if err != nil {
		return err
	}
	if err := r.PutManifest(ctx, env); err != nil {
		return fmt.Errorf("switching the manifest: %w", err)
	}
	report(fmt.Sprintf("published update %d (%d files, %d new)", serial, len(files), len(need)))
	keep := map[string]bool{}
	for _, e := range append(files, live.Files...) {
		keep[e.SHA256] = true
	}
	if err := r.Prune(ctx, keep); err != nil {
		report("warning: couldn't remove old files: " + err.Error())
	}
	return nil
}

func sameFiles(a, b []manifest.Entry) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Path < a[j].Path })
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func keygen(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("not overwriting %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(priv.Seed()) + "\n"); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

func loadKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("key file isn't a base64 ed25519 seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
