package web

import (
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"hufflabs-ro/internal/invite"
	"hufflabs-ro/internal/limit"
	"hufflabs-ro/internal/store"
	"hufflabs-ro/internal/tsapi"
)

const shareURL = "https://login.tailscale.com/admin/invite/abc123"

type fakeStore struct {
	mu        sync.Mutex
	friends   []store.Friend
	nextID    int64
	redeemErr error
	redeemed  []string // "hash|user|md5"
	addErr    error
	revoked   []int64
	accepted  map[int64]bool
	adminErr  error
}

func (f *fakeStore) AddFriend(_ context.Context, name, codeHash, invID, link string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return 0, f.addErr
	}
	f.nextID++
	f.friends = append([]store.Friend{{ID: f.nextID, Name: name, CodeHash: codeHash, ShareInviteID: invID, ShareURL: link, CreatedAt: time.Now()}}, f.friends...)
	return f.nextID, nil
}
func (f *fakeStore) ListFriends(context.Context) ([]store.Friend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.Friend(nil), f.friends...), nil
}
func (f *fakeStore) SetAccepted(_ context.Context, id int64, a bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.accepted == nil {
		f.accepted = map[int64]bool{}
	}
	f.accepted[id] = a
	return nil
}
func (f *fakeStore) Revoke(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = append(f.revoked, id)
	return nil
}
func (f *fakeStore) Redeem(_ context.Context, h, u, p string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.redeemed = append(f.redeemed, h+"|"+u+"|"+p)
	if f.redeemErr != nil {
		return 0, f.redeemErr
	}
	return 2000001, nil
}
func (f *fakeStore) CheckAdmin(_ context.Context, u, p string) (bool, error) {
	if f.adminErr != nil {
		return false, f.adminErr
	}
	return u == "admin" && p == md5hex("adminpass1"), nil
}

type fakeInvites struct {
	findErr error
	list    map[string]tsapi.Invite
	deleted []string
}

func (f *fakeInvites) FindByURL(_ context.Context, link string) (tsapi.Invite, error) {
	if f.findErr != nil {
		return tsapi.Invite{}, f.findErr
	}
	return tsapi.Invite{ID: "inv-1", URL: strings.TrimSpace(link)}, nil
}
func (f *fakeInvites) ListInvites(context.Context) (map[string]tsapi.Invite, error) {
	return f.list, nil
}
func (f *fakeInvites) DeleteInvite(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type env struct {
	srv *httptest.Server
	st  *fakeStore
	inv *fakeInvites
	c   *http.Client
	dl  string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dl := t.TempDir()
	os.WriteFile(filepath.Join(dl, "SHA256SUMS"), []byte("abc  HuffLabsRO-client-2026.11.09.zip\n"), 0o644)
	os.WriteFile(filepath.Join(filepath.Dir(dl), "portal.env"), []byte("SECRET=1"), 0o644)
	st, inv := &fakeStore{}, &fakeInvites{list: map[string]tsapi.Invite{}}
	lim := limit.NewLimiter(5, 10*time.Minute, time.Now)
	up := filepath.Join(filepath.Dir(dl), "update")
	os.MkdirAll(filepath.Join(up, "files"), 0o755)
	os.WriteFile(filepath.Join(up, "manifest.json"), []byte(`{"payload":"x","sigs":[]}`), 0o644)
	os.WriteFile(filepath.Join(up, "files", blobSHA), []byte("0123456789"), 0o644)
	h := New(Config{SiteURL: "https://ro.example.ts.net", DownloadDir: dl, UpdateDir: up, ClientVersion: "2026.11.09",
		SessionKey: []byte("0123456789abcdef0123456789abcdef")}, st, inv, lim)
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	c := srv.Client()
	c.Jar, _ = cookiejar.New(nil)
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &env{srv: srv, st: st, inv: inv, c: c, dl: dl}
}

func (e *env) post(t *testing.T, path string, form url.Values, ip string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	resp, err := e.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, html.UnescapeString(string(b))
}

func (e *env) get(t *testing.T, path string) (int, string, http.Header) {
	t.Helper()
	resp, err := e.c.Get(e.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, html.UnescapeString(string(b)), resp.Header
}

func signupForm(code, user, pw string) url.Values {
	return url.Values{"code": {code}, "username": {user}, "password": {pw}, "confirm": {pw}}
}

var csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

// login logs in as admin and returns the CSRF token from the admin page.
func (e *env) login(t *testing.T) string {
	t.Helper()
	code, _ := e.post(t, "/admin/login", url.Values{"username": {"admin"}, "password": {"adminpass1"}}, "")
	if code != http.StatusSeeOther {
		t.Fatalf("login status %d", code)
	}
	_, body, _ := e.get(t, "/admin")
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no csrf token on admin page")
	}
	return m[1]
}

func TestLandingPage(t *testing.T) {
	e := newEnv(t)
	code, body, h := e.get(t, "/")
	if code != 200 || !strings.Contains(body, "HuffLabs RO") || !strings.Contains(body, "/download/HuffLabsRO-Setup.exe") ||
		!strings.Contains(body, "/download/hufflabs-ro-install.sh") || !strings.Contains(body, `name="code"`) {
		t.Errorf("landing %d missing parts", code)
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Errorf("CSP = %q", h.Get("Content-Security-Policy"))
	}
}

func TestSignupHappy(t *testing.T) {
	e := newEnv(t)
	code, body := e.post(t, "/signup", signupForm("abcd-efgh23", "newplayer", "hunter222"), "100.64.0.5")
	if code != 200 || !strings.Contains(body, "Account created") {
		t.Fatalf("status %d body %s", code, body)
	}
	want := invite.Hash("ABCDEFGH23") + "|newplayer|" + md5hex("hunter222")
	if len(e.st.redeemed) != 1 || e.st.redeemed[0] != want {
		t.Errorf("redeemed = %v, want %s", e.st.redeemed, want)
	}
}

func TestSignupBadCodeCountsTowardLockout(t *testing.T) {
	e := newEnv(t)
	e.st.redeemErr = store.ErrBadCode
	for i := 0; i < 5; i++ {
		code, body := e.post(t, "/signup", signupForm("WRONGWRONG", "newplayer", "hunter222"), "100.64.0.9")
		if code != 200 || !strings.Contains(body, "That invite code isn't valid.") {
			t.Fatalf("try %d: %d", i, code)
		}
	}
	e.st.redeemErr = nil
	code, body := e.post(t, "/signup", signupForm("GOODGOOD22", "newplayer", "hunter222"), "100.64.0.9")
	if code != http.StatusTooManyRequests || !strings.Contains(body, "Too many tries. Wait 10 minutes.") {
		t.Errorf("6th try: %d %q", code, body)
	}
	if code, _ := e.post(t, "/signup", signupForm("GOODGOOD22", "newplayer", "hunter222"), "100.64.0.10"); code != 200 {
		t.Errorf("other IP got %d", code)
	}
}

func TestSignupValidationNoStoreCall(t *testing.T) {
	e := newEnv(t)
	code, body := e.post(t, "/signup", signupForm("ABCDEFGH23", "newplayer", strings.Repeat("p", 24)), "")
	if code != 200 || !strings.Contains(body, "Password must be 8 to 23 characters.") {
		t.Errorf("%d %q", code, body)
	}
	if len(e.st.redeemed) != 0 {
		t.Error("Redeem called for invalid input")
	}
}

func TestSignupTaken(t *testing.T) {
	e := newEnv(t)
	e.st.redeemErr = store.ErrUsernameTaken
	for i := 0; i < 6; i++ {
		code, body := e.post(t, "/signup", signupForm("ABCDEFGH23", "admin", "hunter222"), "100.64.0.7")
		if code != 200 || !strings.Contains(body, "That username is taken.") {
			t.Fatalf("try %d: %d", i, code)
		}
	}
}

func TestSignupKeepsFieldsButNotPassword(t *testing.T) {
	e := newEnv(t)
	e.st.redeemErr = store.ErrUsernameTaken
	_, body := e.post(t, "/signup", signupForm("ABCDEFGH23", "admin", "hunter222"), "")
	if !strings.Contains(body, `value="admin"`) || strings.Contains(body, "hunter222") {
		t.Error("form should keep username and never echo the password")
	}
}

func TestAdminRequiresLogin(t *testing.T) {
	e := newEnv(t)
	code, _, h := e.get(t, "/admin")
	if code != http.StatusSeeOther || h.Get("Location") != "/admin/login" {
		t.Errorf("%d %s", code, h.Get("Location"))
	}
}

func TestAdminBadLogin(t *testing.T) {
	e := newEnv(t)
	code, body := e.post(t, "/admin/login", url.Values{"username": {"admin"}, "password": {"nope12345"}}, "")
	if code != 200 || !strings.Contains(body, "Wrong username or password.") {
		t.Errorf("%d", code)
	}
}

func TestAdminPostNeedsCSRF(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	code, _ := e.post(t, "/admin/friends", url.Values{"name": {"Bob"}, "share_url": {shareURL}}, "")
	if code != http.StatusForbidden {
		t.Errorf("status %d, want 403", code)
	}
}

func TestAddFriendTailscaleFailure(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	e.inv.findErr = tsapi.ErrNotFound
	code, body := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	if code != 200 || !strings.Contains(body, "Tailscale doesn't know that share link") {
		t.Errorf("%d %q", code, body)
	}
	if len(e.st.friends) != 0 {
		t.Error("friend saved despite Tailscale failure")
	}
}

func TestAddFriendRejectsNonTailscaleLink(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	_, body := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {"https://evil.example/x"}}, "")
	if !strings.Contains(body, "Paste the share link from the Tailscale admin console") || len(e.st.friends) != 0 {
		t.Error("non-Tailscale link accepted")
	}
}

func TestMessageContainsCodeOnce(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	code, _ := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	if code != http.StatusSeeOther {
		t.Fatalf("add friend status %d", code)
	}
	_, body, _ := e.get(t, "/admin/friends/1/message")
	for _, want := range []string{"You're invited to HuffLabs RO!", "1. Install Tailscale: https://tailscale.com/download",
		"2. Accept access to the game server: " + shareURL, "3. Open the site (needs Tailscale connected): https://ro.example.ts.net",
		"Setup guide: https://ro.example.ts.net/setup", "(works once)"} {
		if !strings.Contains(body, want) {
			t.Errorf("message missing %q", want)
		}
	}
	m := regexp.MustCompile(`Your invite code: ([A-Z0-9]{10})`).FindStringSubmatch(body)
	if m == nil || invite.Hash(m[1]) != e.st.friends[0].CodeHash {
		t.Fatalf("code on page doesn't match stored hash")
	}
	_, again, _ := e.get(t, "/admin/friends/1/message")
	if strings.Contains(again, m[1]) || !strings.Contains(again, "Code already shown") {
		t.Error("code shown twice")
	}
}

func TestAdminRefreshesAccepted(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	e.inv.list = map[string]tsapi.Invite{"inv-1": {ID: "inv-1", URL: shareURL, Accepted: true}}
	_, body, _ := e.get(t, "/admin")
	if !e.st.accepted[1] || !strings.Contains(body, "Joined Tailscale") {
		t.Errorf("accepted not refreshed: %v", e.st.accepted)
	}
}

func TestRevokeDeletesShareInvite(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	code, _ := e.post(t, "/admin/friends/1/revoke", url.Values{"csrf": {tok}}, "")
	if code != http.StatusSeeOther || len(e.st.revoked) != 1 || len(e.inv.deleted) != 1 || e.inv.deleted[0] != "inv-1" {
		t.Errorf("status %d revoked %v deleted %v", code, e.st.revoked, e.inv.deleted)
	}
}

func TestDownloadServesFile(t *testing.T) {
	e := newEnv(t)
	code, body, _ := e.get(t, "/download/SHA256SUMS")
	if code != 200 || !strings.Contains(body, "HuffLabsRO-client") {
		t.Errorf("%d %q", code, body)
	}
}

func TestDownloadNoTraversal(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/download/..%2Fportal.env", "/download/../portal.env", "/download/", "/download/.hidden"} {
		// ServeMux answers dot-dot paths with a 307 to the cleaned path; that is fine as long
		// as nothing outside DownloadDir is ever served.
		if code, body, _ := e.get(t, p); code == http.StatusOK || strings.Contains(body, "SECRET") {
			t.Errorf("%s -> %d", p, code)
		}
	}
}

var _ = errors.New

// The lockout must key on the address the nearest proxy (tailscale serve) added,
// which is the last X-Forwarded-For entry, not one the client made up.
func TestClientIPUsesLastForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 100.64.0.7")
	if got := clientIP(r); got != "100.64.0.7" {
		t.Errorf("clientIP = %q, want 100.64.0.7", got)
	}
	r.Header.Set("X-Forwarded-For", "100.64.0.8")
	if got := clientIP(r); got != "100.64.0.8" {
		t.Errorf("single entry clientIP = %q", got)
	}
}

func TestAdminLoginDBErrorIsNotABadPassword(t *testing.T) {
	e := newEnv(t)
	e.st.adminErr = errors.New("db down")
	for i := 0; i < 6; i++ {
		code, body := e.post(t, "/admin/login", url.Values{"username": {"admin"}, "password": {"adminpass1"}}, "100.64.0.20")
		if code != http.StatusInternalServerError || !strings.Contains(body, "Something went wrong") {
			t.Fatalf("try %d: %d %q", i, code, body)
		}
	}
	e.st.adminErr = nil
	if code, _ := e.post(t, "/admin/login", url.Values{"username": {"admin"}, "password": {"adminpass1"}}, "100.64.0.20"); code != http.StatusSeeOther {
		t.Errorf("after outage, login got %d (locked out?)", code)
	}
}

func TestAddFriendTailscaleUnreachable(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	e.inv.findErr = errors.New("connection refused")
	_, body := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	if !strings.Contains(body, "Couldn't reach Tailscale to check the share link") || len(e.st.friends) != 0 {
		t.Errorf("body %q, friends %d", body, len(e.st.friends))
	}
}

func TestAddFriendDuplicateLinkFromStore(t *testing.T) {
	e := newEnv(t)
	tok := e.login(t)
	e.st.addErr = store.ErrShareURLUsed
	_, body := e.post(t, "/admin/friends", url.Values{"csrf": {tok}, "name": {"Bob"}, "share_url": {shareURL}}, "")
	if !strings.Contains(body, "That share link is already used") {
		t.Errorf("body %q", body)
	}
}

func TestSetupPage(t *testing.T) {
	e := newEnv(t)
	code, body, _ := e.get(t, "/setup")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"Setup guide", "https://tailscale.com/download", "/download/HuffLabsRO-Setup.exe",
		"/download/hufflabs-ro-install.sh", "More info", "Run anyway", "@menu", "data account settings", `href="/"`} {
		if !strings.Contains(body, want) {
			t.Errorf("setup page missing %q", want)
		}
	}
}

func TestLandingLinksSetup(t *testing.T) {
	e := newEnv(t)
	_, body, _ := e.get(t, "/")
	if !strings.Contains(body, `href="/setup"`) {
		t.Error("landing page has no link to the setup guide")
	}
}

func TestLandingLinuxDownloadIsAButton(t *testing.T) {
	e := newEnv(t)
	_, body, _ := e.get(t, "/")
	if !strings.Contains(body, `class="btn-ghost" href="/download/hufflabs-ro-install.sh"`) || strings.Contains(body, "<details") {
		t.Error("Linux download is not a visible button")
	}
}

const blobSHA = "84d89877f0d4041efb6bf91a16f0248f2fd573e6af05c19f96bedb9f882f7882" // sha256("0123456789")

func TestUpdateManifestServed(t *testing.T) {
	e := newEnv(t)
	code, body, h := e.get(t, "/update/manifest.json")
	if code != 200 || !strings.Contains(body, `"payload"`) || h.Get("Cache-Control") != "no-cache" {
		t.Errorf("%d %q cache=%q", code, body, h.Get("Cache-Control"))
	}
}

func TestUpdateBlobRange(t *testing.T) {
	e := newEnv(t)
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/update/files/"+blobSHA, nil)
	req.Header.Set("Range", "bytes=4-")
	resp, err := e.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusPartialContent || string(b) != "456789" ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "immutable") || resp.Header.Get("Content-Type") != "application/octet-stream" {
		t.Errorf("%d %q %v", resp.StatusCode, b, resp.Header)
	}
}

func TestUpdateBlobRejectsBadNames(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/update/files/..%2Fmanifest.json", "/update/files/" + strings.ToUpper(blobSHA), "/update/files/" + blobSHA[:63],
		"/update/files/manifest.json", "/update/files/", "/update/" + blobSHA} {
		if code, _, _ := e.get(t, p); code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, code)
		}
	}
}

func TestUpdateDisabledWithoutDir(t *testing.T) {
	st, inv := &fakeStore{}, &fakeInvites{list: map[string]tsapi.Invite{}}
	h := New(Config{SiteURL: "https://x", DownloadDir: t.TempDir(), SessionKey: make([]byte, 32)}, st, inv, limit.NewLimiter(5, time.Minute, time.Now))
	for _, p := range []string{"/update/manifest.json", "/update/files/" + blobSHA} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, rec.Code)
		}
	}
}
