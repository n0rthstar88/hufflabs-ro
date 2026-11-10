package store

import (
	"strings"
	"time"

	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"sync"
	"testing"

	"hufflabs-ro/internal/invite"
)

// Integration tests: need PORTAL_TEST_DSN and GAME_TEST_DSN pointing at throwaway
// databases, the game one holding rAthena's `login` table. See deploy/test-store-remote.sh.

func md5hex(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) }

func open(t *testing.T) *Store {
	t.Helper()
	p, g := os.Getenv("PORTAL_TEST_DSN"), os.Getenv("GAME_TEST_DSN")
	if p == "" || g == "" {
		t.Skip("PORTAL_TEST_DSN / GAME_TEST_DSN not set")
	}
	// These tests wipe the friends and login tables, so only ever run them
	// against throwaway ptest_* databases.
	for _, dsn := range []string{p, g} {
		c, err := mysql.ParseDSN(dsn)
		if err != nil || !strings.HasPrefix(c.DBName, "ptest_") {
			t.Fatalf("refusing to run against database %q: test databases must be named ptest_*", c.DBName)
		}
	}
	s, err := Open(p, g)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, q := range []string{"DELETE FROM friends"} {
		if _, err := s.portal.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.game.ExecContext(ctx, "DELETE FROM login"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func addFriend(t *testing.T, s *Store, code string) int64 {
	t.Helper()
	id, err := s.AddFriend(context.Background(), "Friend "+code, invite.Hash(code), "inv-"+code, "https://login.tailscale.com/x/"+code)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func countLogin(t *testing.T, s *Store, where string, args ...any) int {
	t.Helper()
	var n int
	if err := s.game.QueryRow("SELECT COUNT(*) FROM login WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRedeemCreatesAccount(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	fid := addFriend(t, s, "AAAAAAAAAA")
	acct, err := s.Redeem(ctx, invite.Hash("AAAAAAAAAA"), "newplayer", md5hex("hunter222"))
	if err != nil {
		t.Fatal(err)
	}
	var pass, sex string
	var group int
	if err := s.game.QueryRow("SELECT user_pass, sex, group_id FROM login WHERE account_id=?", acct).Scan(&pass, &sex, &group); err != nil {
		t.Fatal(err)
	}
	if pass != md5hex("hunter222") || sex != "M" || group != 0 {
		t.Errorf("row = %q %q %d", pass, sex, group)
	}
	fs, _ := s.ListFriends(ctx)
	if len(fs) != 1 || fs[0].ID != fid || !fs[0].UsedAt.Valid || fs[0].AccountID.Int64 != acct || fs[0].Username.String != "newplayer" {
		t.Errorf("friend after redeem = %+v", fs)
	}
}

func TestRedeemCodeOnce(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	addFriend(t, s, "BBBBBBBBBB")
	if _, err := s.Redeem(ctx, invite.Hash("BBBBBBBBBB"), "first_one", md5hex("x12345678")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Redeem(ctx, invite.Hash("BBBBBBBBBB"), "second_one", md5hex("x12345678")); !errors.Is(err, ErrBadCode) {
		t.Errorf("second redeem err = %v, want ErrBadCode", err)
	}
}

func TestRedeemUnknownCode(t *testing.T) {
	s := open(t)
	if _, err := s.Redeem(context.Background(), invite.Hash("ZZZZZZZZZZ"), "nobody_x", md5hex("x12345678")); !errors.Is(err, ErrBadCode) {
		t.Errorf("err = %v, want ErrBadCode", err)
	}
}

func TestRedeemCaseInsensitiveTaken(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	if _, err := s.game.Exec("INSERT INTO login (userid, user_pass, sex, group_id) VALUES ('admin', ?, 'M', 99)", md5hex("pw")); err != nil {
		t.Fatal(err)
	}
	addFriend(t, s, "CCCCCCCCCC")
	if _, err := s.Redeem(ctx, invite.Hash("CCCCCCCCCC"), "Admin", md5hex("x12345678")); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("err = %v, want ErrUsernameTaken", err)
	}
	fs, _ := s.ListFriends(ctx)
	if fs[0].UsedAt.Valid {
		t.Error("code was spent on a failed redeem")
	}
}

func TestRedeemConcurrent(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	addFriend(t, s, "DDDDDDDDDD")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.Redeem(ctx, invite.Hash("DDDDDDDDDD"), fmt.Sprintf("racer_%d", i), md5hex("x12345678")); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			} else if !errors.Is(err, ErrBadCode) {
				t.Errorf("racer %d: unexpected %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 {
		t.Errorf("successful redeems = %d, want 1", ok)
	}
	if n := countLogin(t, s, "userid LIKE 'racer_%'"); n != 1 {
		t.Errorf("login rows = %d, want 1", n)
	}
}

func TestRedeemConcurrentSameUsername(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	codes := []string{"KKKKKKKKK2", "KKKKKKKKK3", "KKKKKKKKK4", "KKKKKKKKK5", "KKKKKKKKK6", "KKKKKKKKK7", "KKKKKKKKK8", "KKKKKKKKK9"}
	for _, c := range codes {
		addFriend(t, s, c)
	}
	var wg sync.WaitGroup
	for _, c := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			_, err := s.Redeem(ctx, invite.Hash(c), "samename", md5hex("x12345678"))
			if err != nil && !errors.Is(err, ErrUsernameTaken) {
				t.Errorf("%s: unexpected %v", c, err)
			}
		}(c)
	}
	wg.Wait()
	if n := countLogin(t, s, "userid='samename'"); n != 1 {
		t.Errorf("accounts named samename = %d, want 1", n)
	}
}

func TestRevokedCode(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	fid := addFriend(t, s, "EEEEEEEEEE")
	if err := s.Revoke(ctx, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Redeem(ctx, invite.Hash("EEEEEEEEEE"), "late_one", md5hex("x12345678")); !errors.Is(err, ErrRevoked) {
		t.Errorf("err = %v, want ErrRevoked", err)
	}
}

func TestRevokeUsedIsNoop(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	fid := addFriend(t, s, "FFFFFFFFFF")
	if _, err := s.Redeem(ctx, invite.Hash("FFFFFFFFFF"), "used_one", md5hex("x12345678")); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(ctx, fid); err != nil {
		t.Fatal(err)
	}
	fs, _ := s.ListFriends(ctx)
	if fs[0].Revoked {
		t.Error("used invite was marked revoked")
	}
}

func TestSetAccepted(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	fid := addFriend(t, s, "GGGGGGGGGG")
	if err := s.SetAccepted(ctx, fid, true); err != nil {
		t.Fatal(err)
	}
	fs, _ := s.ListFriends(ctx)
	if !fs[0].TailscaleAccepted {
		t.Error("TailscaleAccepted not set")
	}
}

func TestListFriendsNewestFirst(t *testing.T) {
	s := open(t)
	a := addFriend(t, s, "HHHHHHHHHH")
	b := addFriend(t, s, "JJJJJJJJJJ")
	fs, err := s.ListFriends(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 || fs[0].ID != b || fs[1].ID != a {
		t.Errorf("order = %v", fs)
	}
}

func TestCheckAdmin(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	for _, r := range []struct {
		u string
		g int
	}{{"admin", 99}, {"plain", 0}} {
		if _, err := s.game.Exec("INSERT INTO login (userid, user_pass, sex, group_id) VALUES (?, ?, 'M', ?)", r.u, md5hex("goodpass1"), r.g); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		u, p string
		want bool
	}{{"admin", "goodpass1", true}, {"plain", "goodpass1", false}, {"admin", "wrongpass", false}, {"ghost", "goodpass1", false}}
	for _, c := range cases {
		got, err := s.CheckAdmin(ctx, c.u, md5hex(c.p))
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("CheckAdmin(%s,%s) = %v, want %v", c.u, c.p, got, c.want)
		}
	}
}

var _ = sql.ErrNoRows

// A double-clicked "Create account" sends the same code, username and password twice;
// the second must report the same account, not "invite code isn't valid".
func TestRedeemRepeatSameDetailsIsSuccess(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	addFriend(t, s, "GGGGGGGGGG")
	first, err := s.Redeem(ctx, invite.Hash("GGGGGGGGGG"), "clicker", md5hex("x12345678"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Redeem(ctx, invite.Hash("GGGGGGGGGG"), "clicker", md5hex("x12345678"))
	if err != nil || again != first {
		t.Errorf("repeat = %d, %v; want %d, nil", again, err, first)
	}
	if _, err := s.Redeem(ctx, invite.Hash("GGGGGGGGGG"), "clicker", md5hex("other1234")); !errors.Is(err, ErrBadCode) {
		t.Errorf("repeat with other password err = %v, want ErrBadCode", err)
	}
	if n := countLogin(t, s, "userid=?", "clicker"); n != 1 {
		t.Errorf("%d login rows, want 1", n)
	}
}

// The browser giving up mid-request must not strand an account with the code unspent.
func TestRedeemIgnoresRequestCancel(t *testing.T) {
	s := open(t)
	addFriend(t, s, "HHHHHHHHHH")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	acct, err := s.Redeem(ctx, invite.Hash("HHHHHHHHHH"), "quitter", md5hex("x12345678"))
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := s.ListFriends(context.Background())
	if len(fs) != 1 || fs[0].AccountID.Int64 != acct || !fs[0].UsedAt.Valid {
		t.Errorf("friend = %+v, want used with account %d", fs, acct)
	}
}

func TestAddFriendSameShareURLTwice(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	if _, err := s.AddFriend(ctx, "One", invite.Hash("JJJJJJJJJJ"), "inv-1", "https://login.tailscale.com/x/same"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFriend(ctx, "Two", invite.Hash("KKKKKKKKKK"), "inv-1", "https://login.tailscale.com/x/same"); !errors.Is(err, ErrShareURLUsed) {
		t.Errorf("second AddFriend err = %v, want ErrShareURLUsed", err)
	}
}

// Revoke takes the redeem lock, so it can't land between a redeem's checks and
// its update: whichever goes first wins cleanly.
func TestRevokeWaitsForRedeemLock(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	fid := addFriend(t, s, "LLLLLLLLLL")
	conn, err := s.game.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT GET_LOCK('hufflabs_redeem', 1)"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Revoke(ctx, fid) }()
	select {
	case err := <-done:
		t.Fatalf("Revoke returned (%v) while the redeem lock was held", err)
	case <-time.After(500 * time.Millisecond):
	}
	conn.ExecContext(ctx, "SELECT RELEASE_LOCK('hufflabs_redeem')")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	fs, _ := s.ListFriends(ctx)
	if len(fs) != 1 || !fs[0].Revoked {
		t.Errorf("friend = %+v, want revoked", fs)
	}
}
