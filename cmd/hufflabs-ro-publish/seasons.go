package main

// Seasons: Prontera changes with the real season, and the server switches it on its own
// (content/world-round3/server-season-switch.sh), so the publisher's PC doesn't have to be on.
// The server can't sign, so every publish signs one manifest per season, all with the same
// serial: update/seasons/<season>.json. The live update/manifest.json is always a copy of the
// server's current season (update/seasons/current), made on the server under a lock that the
// switch script shares, so a publish and a switch can't put the wrong season live.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hufflabs-ro/internal/manifest"
)

var seasonNames = []string{"spring", "summer", "fall", "winter"}

// SeasonRemote is a Remote that also keeps the per-season manifests.
type SeasonRemote interface {
	Remote
	Season(ctx context.Context) (string, error)                   // the server's current season
	SeasonManifest(ctx context.Context, s string) ([]byte, error) // nil if there is none yet
	PutSeasonManifests(ctx context.Context, m map[string][]byte) error
	ActivateSeason(ctx context.Context) error // manifest.json = seasons/<current>.json
}

// LoadSeasonSets reads <dir>/<season>/... for all four seasons: client-relative path -> file.
func LoadSeasonSets(dir string) (map[string]map[string]string, error) {
	sets := map[string]map[string]string{}
	for _, s := range seasonNames {
		root := filepath.Join(dir, s)
		set := map[string]string{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type().IsRegular() {
				rel, _ := filepath.Rel(root, p)
				set[filepath.ToSlash(rel)] = p
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("season set %s: %w (build it with content/world-round3/build-season-sets.py)", s, err)
		}
		if len(set) == 0 {
			return nil, fmt.Errorf("season set %s is empty", root)
		}
		sets[s] = set
	}
	return sets, nil
}

// SeasonTrees gives one tree per season: the client tree without any file that belongs to
// any season (compared without case: Windows), plus that season's files.
func SeasonTrees(tree map[string]string, sets map[string]map[string]string) map[string]map[string]string {
	managed := map[string]bool{}
	for _, set := range sets {
		for p := range set {
			managed[strings.ToLower(p)] = true
		}
	}
	out := map[string]map[string]string{}
	for s, set := range sets {
		t := map[string]string{}
		for p, src := range tree {
			if !managed[strings.ToLower(p)] {
				t[p] = src
			}
		}
		for p, src := range set {
			t[p] = src
		}
		out[s] = t
	}
	return out
}

func (h *hasher) entries(tree map[string]string, src map[string]string) ([]manifest.Entry, error) {
	paths := make([]string, 0, len(tree))
	for p := range tree {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]manifest.Entry, 0, len(paths))
	for _, p := range paths {
		sum, err := h.sum(tree[p])
		if err != nil {
			return nil, err
		}
		fi, err := os.Stat(tree[p])
		if err != nil {
			return nil, err
		}
		files = append(files, manifest.Entry{Path: p, Size: fi.Size(), SHA256: sum})
		src[sum] = tree[p]
	}
	return files, nil
}

// PublishSeasons is Publish for a client with seasons: it signs all four season manifests
// with one serial, uploads them, then makes the server's current season live.
func PublishSeasons(ctx context.Context, r SeasonRemote, h *hasher, trees map[string]map[string]string, key ed25519.PrivateKey, now time.Time, report func(string)) error {
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
	cur, err := r.Season(ctx)
	if err != nil {
		return fmt.Errorf("reading the server's season: %w", err)
	}
	if trees[cur] == nil {
		return fmt.Errorf("the server's season %q isn't one of %v (update/seasons/current)", cur, seasonNames)
	}
	report("server season: " + cur)

	src := map[string]string{}
	files := map[string][]manifest.Entry{}
	for _, s := range seasonNames {
		if files[s], err = h.entries(trees[s], src); err != nil {
			return err
		}
	}
	if err := h.save(); err != nil {
		report("warning: couldn't save the hash cache: " + err.Error())
	}

	// nothing to do when the live manifest and all four season manifests already match
	upToDate := sameFiles(live.Files, files[cur])
	for _, s := range seasonNames {
		if !upToDate {
			break
		}
		b, err := r.SeasonManifest(ctx, s)
		if err != nil || b == nil {
			upToDate = false
			break
		}
		m, err := manifest.Verify(b, []ed25519.PublicKey{pub})
		upToDate = err == nil && m.Serial == live.Serial && sameFiles(m.Files, files[s])
	}
	if upToDate {
		report("Nothing changed; the server already has this.")
		return nil
	}

	have, err := r.HaveBlobs(ctx)
	if err != nil {
		return err
	}
	need := map[string]string{}
	var bytesUp int64
	for _, s := range seasonNames {
		for _, e := range files[s] {
			if !have[e.SHA256] && need[e.SHA256] == "" {
				need[e.SHA256] = src[e.SHA256]
				bytesUp += e.Size
			}
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
	envs := map[string][]byte{}
	for _, s := range seasonNames {
		if envs[s], err = manifest.Sign(manifest.Manifest{Serial: serial, Created: now.UTC(), Files: files[s]}, key); err != nil {
			return err
		}
	}
	if err := r.PutSeasonManifests(ctx, envs); err != nil {
		return fmt.Errorf("uploading the season manifests: %w", err)
	}
	if err := r.ActivateSeason(ctx); err != nil {
		return fmt.Errorf("switching the manifest: %w", err)
	}
	report(fmt.Sprintf("published update %d (%d files live for %s, %d new; all 4 seasons signed)", serial, len(files[cur]), cur, len(need)))
	keep := map[string]bool{}
	for _, e := range live.Files {
		keep[e.SHA256] = true
	}
	for _, s := range seasonNames {
		for _, e := range files[s] {
			keep[e.SHA256] = true
		}
	}
	if err := r.Prune(ctx, keep); err != nil {
		report("warning: couldn't remove old files: " + err.Error())
	}
	return nil
}

// seasonLock is shared with the server's switch script (flock on update/seasons/.lock).
const seasonLock = `exec 9>"$D/seasons/.lock"; flock -w 120 9`

func (r *scriptRemote) Season(ctx context.Context) (string, error) {
	var out bytes.Buffer
	if err := r.run(ctx, `cat `+shq(r.dir+"/seasons/current")+` 2>/dev/null || true`, nil, &out); err != nil {
		return "", err
	}
	s := strings.TrimSpace(out.String())
	if s == "" {
		return "", errors.New("the server has no update/seasons/current yet (install content/world-round3/server-season-switch.sh)")
	}
	return s, nil
}

func (r *scriptRemote) SeasonManifest(ctx context.Context, s string) ([]byte, error) {
	var out bytes.Buffer
	if err := r.run(ctx, `f=`+shq(r.dir+"/seasons/"+s+".json")+`; if [ -f "$f" ]; then cat "$f"; fi`, nil, &out); err != nil {
		return nil, err
	}
	if out.Len() == 0 {
		return nil, nil
	}
	return out.Bytes(), nil
}

// PutSeasonManifests uploads each as seasons/<s>.json.new, checks every hash, then moves all
// of them into place under the lock.
func (r *scriptRemote) PutSeasonManifests(ctx context.Context, m map[string][]byte) error {
	for s, b := range m {
		script := `set -e
D=` + shq(r.dir) + `
mkdir -p "$D/seasons"
cat > "$D/seasons/` + s + `.json.new"
if [ "$(sha256sum "$D/seasons/` + s + `.json.new" | cut -d' ' -f1)" != ` + shq(sha256hex(b)) + ` ]; then
	rm -f "$D/seasons/` + s + `.json.new"; echo "season manifest upload was cut off" >&2; exit 1
fi
chmod 644 "$D/seasons/` + s + `.json.new"`
		if err := r.run(ctx, script, bytes.NewReader(b), io.Discard); err != nil {
			return err
		}
	}
	script := `set -e
D=` + shq(r.dir) + `
` + seasonLock + `
for s in ` + strings.Join(seasonNames, " ") + `; do mv -f "$D/seasons/$s.json.new" "$D/seasons/$s.json"; done`
	return r.run(ctx, script, nil, io.Discard)
}

func (r *scriptRemote) ActivateSeason(ctx context.Context) error {
	script := `set -e
D=` + shq(r.dir) + `
` + seasonLock + `
cur=$(cat "$D/seasons/current")
cp "$D/seasons/$cur.json" "$D/manifest.json.new"
chmod 644 "$D/manifest.json.new"; set -- "$D/manifest.json.new"; ` + setGroup + `
mv -f "$D/manifest.json.new" "$D/manifest.json"`
	return r.run(ctx, script, nil, io.Discard)
}

// drySeasonRemote reads from the server but changes nothing.
type drySeasonRemote struct{ SeasonRemote }

func (d drySeasonRemote) PutBlobs(_ context.Context, b map[string]string) error {
	fmt.Printf("(dry run) would upload %d files\n", len(b))
	return nil
}
func (d drySeasonRemote) PutManifest(context.Context, []byte) error    { return nil }
func (d drySeasonRemote) Prune(context.Context, map[string]bool) error { return nil }
func (d drySeasonRemote) PutSeasonManifests(context.Context, map[string][]byte) error {
	fmt.Println("(dry run) would upload the 4 season manifests")
	return nil
}
func (d drySeasonRemote) ActivateSeason(context.Context) error {
	fmt.Println("(dry run) would switch the manifest to the server's season")
	return nil
}
