// Package web serves the HuffLabs RO sign-up site and the admin pages.
package web

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"hufflabs-ro/internal/invite"
	"hufflabs-ro/internal/limit"
	"hufflabs-ro/internal/store"
	"hufflabs-ro/internal/tsapi"
	"hufflabs-ro/internal/validate"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Store is the subset of *store.Store the handlers use.
type Store interface {
	AddFriend(ctx context.Context, name, codeHash, shareInviteID, shareURL string) (int64, error)
	ListFriends(ctx context.Context) ([]store.Friend, error)
	SetAccepted(ctx context.Context, friendID int64, accepted bool) error
	Revoke(ctx context.Context, friendID int64) error
	Redeem(ctx context.Context, codeHash, username, md5Pass string) (int64, error)
	CheckAdmin(ctx context.Context, username, md5Pass string) (bool, error)
}

// Invites is the subset of *tsapi.Client the handlers use.
type Invites interface {
	FindByURL(ctx context.Context, link string) (tsapi.Invite, error)
	ListInvites(ctx context.Context) (map[string]tsapi.Invite, error)
	DeleteInvite(ctx context.Context, id string) error
}

type Config struct {
	SiteURL       string
	DownloadDir   string
	UpdateDir     string // launcher updates: manifest.json and files/<sha256>; empty disables /update/
	ClientVersion string
	SessionKey    []byte
}

const (
	cookieName  = "hlro_admin"
	sessionLife = 8 * time.Hour
	shareURLPre = "https://login.tailscale.com/"
)

type server struct {
	cfg  Config
	st   Store
	inv  Invites
	lim  *limit.Limiter
	tmpl map[string]*template.Template

	mu      sync.Mutex
	pending map[int64]string // plain invite codes not yet shown, by friend ID
}

func New(cfg Config, st Store, inv Invites, lim *limit.Limiter) http.Handler {
	s := &server{cfg: cfg, st: st, inv: inv, lim: lim, pending: map[int64]string{}, tmpl: map[string]*template.Template{}}
	// Each page is parsed with the shared layout on its own, so per-page blocks don't collide.
	for _, page := range []string{"landing.html", "signup_ok.html", "admin_login.html", "admin.html", "message.html"} {
		s.tmpl[page] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+page))
	}
	static, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.landing)
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("GET /admin/login", s.adminLoginPage)
	mux.HandleFunc("POST /admin/login", s.adminLogin)
	mux.HandleFunc("POST /admin/logout", s.admin(s.adminLogout))
	mux.HandleFunc("GET /admin", s.admin(s.adminHome))
	mux.HandleFunc("POST /admin/friends", s.admin(s.addFriend))
	mux.HandleFunc("POST /admin/friends/{id}/revoke", s.admin(s.revoke))
	mux.HandleFunc("GET /admin/friends/{id}/message", s.admin(s.message))
	mux.HandleFunc("GET /download/{file}", s.download)
	if cfg.UpdateDir != "" {
		mux.HandleFunc("GET /update/manifest.json", s.updateManifest)
		mux.HandleFunc("GET /update/files/{sha}", s.updateFile)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return headers(mux)
}

func headers(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}

func (s *server) render(w http.ResponseWriter, status int, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["SiteURL"] = s.cfg.SiteURL
	data["Version"] = s.cfg.ClientVersion
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl[name].ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template", "name", name, "err", err)
	}
}

// clientIP is the last X-Forwarded-For address, the one the nearest proxy (tailscale
// serve) added; earlier entries come from the client and can be made up. tailscale
// serve replaces the header anyway (checked 2026-11-09). Else RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func md5hex(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) }

// ---- public pages ----

func (s *server) landing(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "landing.html", nil)
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	code, user := r.PostFormValue("code"), strings.TrimSpace(r.PostFormValue("username"))
	pw, confirm := r.PostFormValue("password"), r.PostFormValue("confirm")
	fail := func(status int, msg string) {
		s.render(w, status, "landing.html", map[string]any{"Error": msg, "Code": code, "Username": user})
	}
	if s.lim.Blocked(ip) {
		slog.Info("signup locked", "ip", ip)
		fail(http.StatusTooManyRequests, "Too many tries. Wait 10 minutes.")
		return
	}
	if err := validate.Username(user); err != nil {
		fail(http.StatusOK, err.Error())
		return
	}
	if err := validate.Password(pw, confirm); err != nil {
		fail(http.StatusOK, err.Error())
		return
	}
	if validate.NormalizeCode(code) == "" {
		fail(http.StatusOK, "Enter the invite code from your invite message.")
		return
	}
	acct, err := s.st.Redeem(r.Context(), invite.Hash(code), user, md5hex(pw))
	switch {
	case errors.Is(err, store.ErrBadCode), errors.Is(err, store.ErrRevoked):
		s.lim.Fail(ip)
		slog.Info("signup bad_code", "ip", ip)
		fail(http.StatusOK, "That invite code isn't valid.")
	case errors.Is(err, store.ErrUsernameTaken):
		fail(http.StatusOK, "That username is taken.")
	case err != nil:
		slog.Error("signup", "err", err)
		fail(http.StatusInternalServerError, "Something went wrong. Try again in a minute.")
	default:
		slog.Info("signup ok", "user", user, "account", acct)
		s.render(w, http.StatusOK, "signup_ok.html", map[string]any{"Username": user})
	}
}

// download serves a file from DownloadDir; ServeContent supports Range, so
// installers can resume an interrupted download.
func (s *server) download(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\`) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.cfg.DownloadDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, fi.ModTime(), f)
}

// ---- launcher updates ----

func (s *server) updateManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	s.serveUpdate(w, r, filepath.Join(s.cfg.UpdateDir, "manifest.json"), "application/json")
}

// updateFile serves one content-addressed file; the name is its SHA-256, so it
// never changes and can be cached for good. ServeContent handles Range (resume).
func (s *server) updateFile(w http.ResponseWriter, r *http.Request) {
	sha := r.PathValue("sha")
	if !isSHA256Hex(sha) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	s.serveUpdate(w, r, filepath.Join(s.cfg.UpdateDir, "files", sha), "application/octet-stream")
}

func (s *server) serveUpdate(w http.ResponseWriter, r *http.Request, path, ctype string) {
	f, err := os.Open(path)
	if err != nil {
		w.Header().Del("Cache-Control")
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		w.Header().Del("Cache-Control")
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func isSHA256Hex(s string) bool {
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

// ---- admin session ----

type session struct{ user, csrf string }

func (s *server) sign(payload string) string {
	m := hmac.New(sha256.New, s.cfg.SessionKey)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *server) setSession(w http.ResponseWriter, user string) {
	b := make([]byte, 18)
	rand.Read(b)
	csrf := base64.RawURLEncoding.EncodeToString(b)
	exp := strconv.FormatInt(time.Now().Add(sessionLife).Unix(), 10)
	payload := exp + "|" + csrf + "|" + user
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + s.sign(payload),
		Path: "/admin", MaxAge: int(sessionLife.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}

func (s *server) getSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return session{}, false
	}
	enc, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		return session{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || !hmac.Equal([]byte(sig), []byte(s.sign(string(raw)))) {
		return session{}, false
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return session{}, false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return session{}, false
	}
	return session{csrf: parts[1], user: parts[2]}, true
}

type adminHandler func(w http.ResponseWriter, r *http.Request, sess session)

// admin requires a session, and a matching CSRF token on POSTs.
func (s *server) admin(h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.getSession(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost && !hmac.Equal([]byte(r.PostFormValue("csrf")), []byte(sess.csrf)) {
			http.Error(w, "Form expired. Go back, reload the page and try again.", http.StatusForbidden)
			return
		}
		h(w, r, sess)
	}
}

func (s *server) adminLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "admin_login.html", nil)
}

func (s *server) adminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.lim.Blocked(ip) {
		s.render(w, http.StatusTooManyRequests, "admin_login.html", map[string]any{"Error": "Too many tries. Wait 10 minutes."})
		return
	}
	user := strings.TrimSpace(r.PostFormValue("username"))
	ok, err := s.st.CheckAdmin(r.Context(), user, md5hex(r.PostFormValue("password")))
	if err != nil {
		// a database problem isn't a wrong password, so it doesn't count toward lockout
		slog.Error("admin login", "err", err)
		s.render(w, http.StatusInternalServerError, "admin_login.html", map[string]any{"Error": "Something went wrong. Try again in a minute."})
		return
	}
	if !ok {
		s.lim.Fail(ip)
		slog.Info("admin login failed", "ip", ip)
		s.render(w, http.StatusOK, "admin_login.html", map[string]any{"Error": "Wrong username or password."})
		return
	}
	s.setSession(w, user)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *server) adminLogout(w http.ResponseWriter, r *http.Request, _ session) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/admin", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// ---- admin pages ----

type friendRow struct {
	store.Friend
	Status string
}

func status(f store.Friend) string {
	switch {
	case f.UsedAt.Valid:
		return "Account created (" + f.Username.String + ")"
	case f.Revoked:
		return "Revoked"
	case f.TailscaleAccepted:
		return "Joined Tailscale"
	default:
		return "Invited"
	}
}

func (s *server) adminPage(w http.ResponseWriter, r *http.Request, sess session, data map[string]any) {
	ctx := r.Context()
	friends, err := s.st.ListFriends(ctx)
	if err != nil {
		slog.Error("list friends", "err", err)
	}
	invs, ierr := s.inv.ListInvites(ctx)
	if ierr != nil {
		slog.Warn("list invites", "err", ierr)
		data["Warning"] = "Couldn't reach Tailscale, so the \"Joined Tailscale\" status may be out of date."
	}
	rows := make([]friendRow, 0, len(friends))
	for _, f := range friends {
		if inv, ok := invs[f.ShareInviteID]; ierr == nil && ok && inv.Accepted != f.TailscaleAccepted {
			if err := s.st.SetAccepted(ctx, f.ID, inv.Accepted); err == nil {
				f.TailscaleAccepted = inv.Accepted
			}
		}
		rows = append(rows, friendRow{Friend: f, Status: status(f)})
	}
	data["Friends"], data["CSRF"], data["User"] = rows, sess.csrf, sess.user
	s.render(w, http.StatusOK, "admin.html", data)
}

func (s *server) adminHome(w http.ResponseWriter, r *http.Request, sess session) {
	s.adminPage(w, r, sess, map[string]any{})
}

func (s *server) addFriend(w http.ResponseWriter, r *http.Request, sess session) {
	ctx := r.Context()
	name, link := strings.TrimSpace(r.PostFormValue("name")), strings.TrimSpace(r.PostFormValue("share_url"))
	formErr := func(msg string) {
		s.adminPage(w, r, sess, map[string]any{"Error": msg, "Name": name, "ShareURL": link})
	}
	if name == "" || len([]rune(name)) > 64 {
		formErr("Give the friend a name (up to 64 characters).")
		return
	}
	if !strings.HasPrefix(link, shareURLPre) {
		formErr("Paste the share link from the Tailscale admin console (Machines → ragnarok → Share).")
		return
	}
	if friends, err := s.st.ListFriends(ctx); err == nil {
		for _, f := range friends {
			if f.ShareURL == link {
				formErr("That share link is already used for " + f.Name + ". Make a new one for each friend.")
				return
			}
		}
	}
	inv, err := s.inv.FindByURL(ctx, link)
	if errors.Is(err, tsapi.ErrNotFound) {
		formErr("Tailscale doesn't know that share link for the ragnarok machine. Check you copied it from ragnarok's Share dialog.")
		return
	} else if err != nil {
		slog.Error("find invite", "err", err)
		formErr("Couldn't reach Tailscale to check the share link. Try again in a minute.")
		return
	}
	code, err := invite.New()
	if err != nil {
		formErr("Couldn't make an invite code. Try again.")
		return
	}
	id, err := s.st.AddFriend(ctx, name, invite.Hash(code), inv.ID, link)
	if errors.Is(err, store.ErrShareURLUsed) {
		formErr("That share link is already used for another friend. Make a new one for each friend.")
		return
	} else if err != nil {
		slog.Error("add friend", "err", err)
		formErr("Couldn't save the friend. Try again.")
		return
	}
	s.mu.Lock()
	s.pending[id] = code
	s.mu.Unlock()
	slog.Info("friend added", "id", id)
	http.Redirect(w, r, "/admin/friends/"+strconv.FormatInt(id, 10)+"/message", http.StatusSeeOther)
}

func (s *server) findFriend(ctx context.Context, idStr string) (store.Friend, bool) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return store.Friend{}, false
	}
	friends, err := s.st.ListFriends(ctx)
	if err != nil {
		return store.Friend{}, false
	}
	for _, f := range friends {
		if f.ID == id {
			return f, true
		}
	}
	return store.Friend{}, false
}

func (s *server) message(w http.ResponseWriter, r *http.Request, sess session) {
	f, ok := s.findFriend(r.Context(), r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	code, fresh := s.pending[f.ID]
	delete(s.pending, f.ID)
	s.mu.Unlock()
	s.render(w, http.StatusOK, "message.html", map[string]any{"Friend": f, "Code": code, "Fresh": fresh, "CSRF": sess.csrf})
}

func (s *server) revoke(w http.ResponseWriter, r *http.Request, _ session) {
	ctx := r.Context()
	f, ok := s.findFriend(ctx, r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !f.UsedAt.Valid {
		if err := s.st.Revoke(ctx, f.ID); err != nil {
			slog.Error("revoke", "err", err)
		}
		if f.ShareInviteID != "" {
			if err := s.inv.DeleteInvite(ctx, f.ShareInviteID); err != nil {
				slog.Warn("delete share invite", "id", f.ShareInviteID, "err", err)
			}
		}
		slog.Info("friend revoked", "id", f.ID)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
