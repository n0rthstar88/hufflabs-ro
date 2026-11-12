package web

import (
	"database/sql"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hufflabs-ro/internal/store"
	"hufflabs-ro/internal/tsapi"
)

// TestWriteScreenshotPages saves the sign-up, admin and invite-message pages, with
// made-up friends, as static HTML for README screenshots when HLRO_SHOTS is a folder.
func TestWriteScreenshotPages(t *testing.T) {
	out := os.Getenv("HLRO_SHOTS")
	if out == "" {
		t.Skip("set HLRO_SHOTS to a folder to write the pages")
	}
	e := newEnv(t)
	now := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	e.st.friends = []store.Friend{
		{ID: 3, Name: "Rowan", CreatedAt: now, ShareInviteID: "inv-3"},
		{ID: 2, Name: "Mira", CreatedAt: now.Add(-time.Hour), TailscaleAccepted: true, ShareInviteID: "inv-2"},
		{ID: 1, Name: "Theo", CreatedAt: now.Add(-26 * time.Hour), TailscaleAccepted: true, ShareInviteID: "inv-1",
			UsedAt: sql.NullTime{Time: now, Valid: true}, Username: sql.NullString{String: "theo_knight", Valid: true}, AccountID: sql.NullInt64{Int64: 2000002, Valid: true}},
	}
	e.st.nextID = 3
	e.inv.list = map[string]tsapi.Invite{"inv-1": {ID: "inv-1", Accepted: true}, "inv-2": {ID: "inv-2", Accepted: true}}
	save := func(name, body string) {
		body = strings.ReplaceAll(body, `"/static/`, `"static/`)
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, landing, _ := e.get(t, "/")
	save("signup.html", landing)
	_, setup, _ := e.get(t, "/setup")
	save("setup.html", setup)
	tok := e.login(t)
	_, admin, _ := e.get(t, "/admin")
	save("admin.html", admin)
	code, _ := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Juniper"}, "share_url": {shareURL}}, "")
	if code != 303 {
		t.Fatalf("add friend %d", code)
	}
	_, msg, _ := e.get(t, "/admin/friends/4/message")
	save("message.html", msg)
	static, _ := fs.Sub(staticFS, "static")
	os.MkdirAll(filepath.Join(out, "static"), 0o755)
	fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := fs.ReadFile(static, p)
			b = []byte(strings.ReplaceAll(string(b), "url(/static/", "url(")) // relative for file://
			os.WriteFile(filepath.Join(out, "static", p), b, 0o644)
		}
		return nil
	})
}
