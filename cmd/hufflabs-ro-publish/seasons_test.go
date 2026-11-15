package main

import (
	"context"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"hufflabs-ro/internal/manifest"
)

type fakeSeasonRemote struct {
	fakeRemote
	season  string
	seasons map[string][]byte
}

func (f *fakeSeasonRemote) Season(context.Context) (string, error) { return f.season, nil }
func (f *fakeSeasonRemote) SeasonManifest(_ context.Context, s string) ([]byte, error) {
	return f.seasons[s], nil
}
func (f *fakeSeasonRemote) PutSeasonManifests(_ context.Context, m map[string][]byte) error {
	f.calls = append(f.calls, "PutSeasonManifests")
	f.seasons = m
	return nil
}
func (f *fakeSeasonRemote) ActivateSeason(context.Context) error {
	f.calls = append(f.calls, "ActivateSeason")
	f.live = f.seasons[f.season]
	return nil
}

func seasonSets(t *testing.T) map[string]map[string]string {
	return map[string]map[string]string{
		"spring": tree(t, map[string]string{"data/prontera.gat": "spring gat", "data/texture/flower.bmp": "f"}),
		"summer": tree(t, map[string]string{"data/prontera.gat": "summer gat"}),
		"fall":   tree(t, map[string]string{"data/prontera.gat": "fall gat", "data/texture/Leaf.bmp": "l"}),
		"winter": tree(t, map[string]string{"data/prontera.gat": "winter gat", "data/texture/snow.bmp": "s"}),
	}
}

func TestSeasonTreesSwapOnlySeasonFiles(t *testing.T) {
	// the client folder holds fall, with Leaf.bmp spelled in lower case
	client := tree(t, map[string]string{"data/prontera.gat": "fall gat", "data/texture/leaf.bmp": "l", "a.txt": "a"})
	trees := SeasonTrees(client, seasonSets(t))
	w := trees["winter"]
	if _, ok := w["data/texture/leaf.bmp"]; ok {
		t.Error("winter kept fall's leaf.bmp (case-insensitive match missed)")
	}
	if _, ok := w["data/texture/snow.bmp"]; !ok {
		t.Error("winter lacks snow.bmp")
	}
	if _, ok := w["a.txt"]; !ok {
		t.Error("winter lost a non-season file")
	}
	if len(trees["summer"]) != 2 {
		t.Errorf("summer = %v, want a.txt + prontera.gat", trees["summer"])
	}
}

func TestPublishSeasonsSignsAllAndActivatesServerSeason(t *testing.T) {
	pub, priv := keyPair(t)
	client := tree(t, map[string]string{"data/prontera.gat": "fall gat", "a.txt": "a"})
	r := &fakeSeasonRemote{fakeRemote: fakeRemote{blobs: map[string]bool{}}, season: "winter"}
	h := newHasher(t)
	now := time.Unix(2_000_000_000, 0)
	if err := PublishSeasons(context.Background(), r, h, SeasonTrees(client, seasonSets(t)), priv, now, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls, ","); !strings.Contains(got, "PutBlobs,PutSeasonManifests,ActivateSeason,Prune") {
		t.Errorf("call order %s", got)
	}
	var serial int64
	for _, s := range seasonNames {
		m, err := manifest.Verify(r.seasons[s], []ed25519.PublicKey{pub})
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if serial == 0 {
			serial = m.Serial
		} else if m.Serial != serial {
			t.Errorf("%s serial %d, want %d (all seasons share one)", s, m.Serial, serial)
		}
	}
	live, _ := manifest.Verify(r.live, []ed25519.PublicKey{pub})
	for _, e := range live.Files {
		if e.Path == "data/prontera.gat" && e.Size != int64(len("winter gat")) {
			t.Error("live manifest isn't the server's season (winter)")
		}
	}
	// publishing the same thing again changes nothing
	r.calls = nil
	for _, e := range live.Files {
		r.blobs[e.SHA256] = true
	}
	if err := PublishSeasons(context.Background(), r, h, SeasonTrees(client, seasonSets(t)), priv, now.Add(time.Hour), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(r.calls, ","), "Put") {
		t.Errorf("second publish uploaded: %v", r.calls)
	}
}

func TestPublishSeasonsRefusesUnknownServerSeason(t *testing.T) {
	_, priv := keyPair(t)
	r := &fakeSeasonRemote{fakeRemote: fakeRemote{blobs: map[string]bool{}}, season: "monsoon"}
	err := PublishSeasons(context.Background(), r, newHasher(t), SeasonTrees(tree(t, map[string]string{"a": "a"}), seasonSets(t)), priv, time.Now(), func(string) {})
	if err == nil {
		t.Fatal("published for an unknown season")
	}
}
