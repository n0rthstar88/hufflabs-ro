// Command hufflabs-ro-publish pushes the publisher's changed game files to the server so
// every launcher picks them up on its next start.
//
//	hufflabs-ro-publish keygen [--out FILE]
//	hufflabs-ro-publish [--client DIR] [--repo DIR] [--key FILE] [--dry-run]
//
// The server is reached with ssh $HLRO_PROXMOX "pct exec $HLRO_CT …"; the update
// folder is $HLRO_UPDATE_DIR (default /srv/hufflabs-portal/update).
package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	home, _ := os.UserHomeDir()
	defKey := filepath.Join(home, ".config", "hufflabs-ro", "publish.key")
	if len(os.Args) > 1 && os.Args[1] == "keygen" {
		fs := flag.NewFlagSet("keygen", flag.ExitOnError)
		out := fs.String("out", defKey, "where to write the private key")
		fs.Parse(os.Args[2:])
		pub, err := keygen(*out)
		if err != nil {
			fail(err)
		}
		fmt.Printf("private key: %s (keep it secret, back it up)\npublic key:  %s\n", *out, pub)
		return
	}
	cwd, _ := os.Getwd()
	defClient := filepath.Join(home, "Games", "Ragnarok", "Client")
	if _, err := os.Stat(filepath.Join(cwd, "data.grf")); err == nil {
		defClient = cwd
	}
	client := flag.String("client", defClient, "the client folder (default: this folder if it holds data.grf)")
	repo := flag.String("repo", envOr("HLRO_REPO", filepath.Join(home, "Projects", "hufflabs-ro")), "the hufflabs-ro checkout (theme/out, client/, build/launcher)")
	keyFile := flag.String("key", defKey, "private signing key")
	dry := flag.Bool("dry-run", false, "work out what would change, upload nothing")
	list := flag.Bool("list", false, "print the files that would ship and exit")
	flag.BoolVar(&forceKeyChange, "force-key-change", false, "replace a live manifest signed by a different key")
	flag.Parse()

	key, err := loadKey(*keyFile)
	if err != nil {
		fail(fmt.Errorf("signing key %s: %w (run: hufflabs-ro-publish keygen)", *keyFile, err))
	}
	pats, err := readExcludes(filepath.Join(*repo, "client", "excludes.txt"))
	if err != nil {
		fail(err)
	}
	cache := filepath.Join(home, ".cache", "hufflabs-ro")
	siteURL := os.Getenv("HLRO_SITE_URL")
	if !strings.HasPrefix(siteURL, "https://") {
		fail(fmt.Errorf("set HLRO_SITE_URL to the site's https:// address (used for the in-game Sign Up button)"))
	}
	tree, err := BuildTree(*client, filepath.Join(*repo, "theme", "out"), filepath.Join(*repo, "client", "defaults"),
		filepath.Join(*repo, "build", "launcher", "HuffLabsRO-Launcher.exe"), pats, filepath.Join(cache, "gen"), siteURL)
	if err != nil {
		fail(err)
	}
	if *list {
		paths := make([]string, 0, len(tree))
		for p := range tree {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Println(p)
		}
		return
	}
	if err := checkLauncherBuild(tree["HuffLabsRO-Launcher.exe"], key.Public().(ed25519.PublicKey), siteURL); err != nil {
		fail(err)
	}
	proxmox, ct := os.Getenv("HLRO_PROXMOX"), os.Getenv("HLRO_CT")
	if proxmox == "" || ct == "" {
		fail(fmt.Errorf("set HLRO_PROXMOX (ssh target) and HLRO_CT (container id)"))
	}
	var r Remote = &scriptRemote{run: sshRun(proxmox, ct), dir: envOr("HLRO_UPDATE_DIR", "/srv/hufflabs-portal/update")}
	if *dry {
		r = dryRemote{r}
	}
	h := &hasher{path: filepath.Join(cache, "hashes.json")}
	ctx := context.Background()
	report := func(s string) { fmt.Println(s) }
	if *dry {
		report = func(s string) { fmt.Println("(dry run) " + s) }
	}
	if err := Publish(ctx, r, h, tree, key, time.Now(), report); err != nil {
		fail(err)
	}
}

// dryRemote reads from the server but changes nothing.
type dryRemote struct{ Remote }

func (d dryRemote) PutBlobs(_ context.Context, b map[string]string) error {
	fmt.Printf("(dry run) would upload %d files\n", len(b))
	return nil
}
func (d dryRemote) PutManifest(context.Context, []byte) error {
	fmt.Println("(dry run) would switch the manifest")
	return nil
}
func (d dryRemote) Prune(context.Context, map[string]bool) error { return nil }

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hufflabs-ro-publish:", err)
	os.Exit(1)
}
