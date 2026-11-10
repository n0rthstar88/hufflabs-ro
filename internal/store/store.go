// Package store is the portal's database access: its own friends/invites table in
// hufflabs_portal, and account creation in rAthena's ragnarok.login table.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed schema.sql
var schema string

var (
	ErrBadCode       = errors.New("invite code not valid")
	ErrUsernameTaken = errors.New("username taken")
	ErrRevoked       = errors.New("invite revoked")
	ErrShareURLUsed  = errors.New("share link already used for another friend")
)

// Friend is one invited friend and the state of their invite.
type Friend struct {
	ID                int64
	Name              string
	CodeHash          string
	ShareInviteID     string
	ShareURL          string
	CreatedAt         time.Time
	TailscaleAccepted bool
	AccountID         sql.NullInt64
	Username          sql.NullString
	UsedAt            sql.NullTime
	Revoked           bool
}

type Store struct {
	portal *sql.DB
	game   *sql.DB
}

// Open connects to both databases and applies the portal schema.
func Open(portalDSN, gameDSN string) (*Store, error) {
	p, err := sql.Open("mysql", portalDSN)
	if err != nil {
		return nil, err
	}
	g, err := sql.Open("mysql", gameDSN)
	if err != nil {
		p.Close()
		return nil, err
	}
	s := &Store{portal: p, game: g}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := g.PingContext(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("game db: %w", err)
	}
	if _, err := p.ExecContext(ctx, schema); err != nil {
		s.Close()
		return nil, fmt.Errorf("portal schema: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	s.game.Close()
	return s.portal.Close()
}

// lock takes the server-wide named lock that serializes every invite change
// (redeem, add, revoke). Call the returned func to release it.
func (s *Store) lock(ctx context.Context) (*sql.Conn, func(), error) {
	conn, err := s.game.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('hufflabs_redeem', 10)").Scan(&got); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		conn.Close()
		return nil, nil, errors.New("could not get redeem lock")
	}
	return conn, func() {
		conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('hufflabs_redeem')")
		conn.Close()
	}, nil
}

// AddFriend saves a new invite. A share link can belong to only one friend.
func (s *Store) AddFriend(ctx context.Context, name, codeHash, shareInviteID, shareURL string) (int64, error) {
	_, unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()
	var one int
	err = s.portal.QueryRowContext(ctx, "SELECT 1 FROM friends WHERE share_url=? LIMIT 1", shareURL).Scan(&one)
	if err == nil {
		return 0, ErrShareURLUsed
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	r, err := s.portal.ExecContext(ctx,
		"INSERT INTO friends (name, code_hash, share_invite_id, share_url) VALUES (?, ?, ?, ?)",
		name, codeHash, shareInviteID, shareURL)
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// ListFriends returns all friends, newest first.
func (s *Store) ListFriends(ctx context.Context) ([]Friend, error) {
	rows, err := s.portal.QueryContext(ctx, `SELECT id, name, code_hash, COALESCE(share_invite_id,''), COALESCE(share_url,''),
		created_at, ts_accepted, account_id, username, used_at, revoked FROM friends ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Friend
	for rows.Next() {
		var f Friend
		if err := rows.Scan(&f.ID, &f.Name, &f.CodeHash, &f.ShareInviteID, &f.ShareURL, &f.CreatedAt,
			&f.TailscaleAccepted, &f.AccountID, &f.Username, &f.UsedAt, &f.Revoked); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) SetAccepted(ctx context.Context, friendID int64, accepted bool) error {
	_, err := s.portal.ExecContext(ctx, "UPDATE friends SET ts_accepted=? WHERE id=?", accepted, friendID)
	return err
}

// Revoke marks an unused invite revoked; a used invite is left alone.
// It waits for any redeem in progress, so the two can't interleave.
func (s *Store) Revoke(ctx context.Context, friendID int64) error {
	_, unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = s.portal.ExecContext(ctx, "UPDATE friends SET revoked=1 WHERE id=? AND used_at IS NULL", friendID)
	return err
}

// Redeem spends an invite code and creates the rAthena account. The login table is
// MyISAM (no transactions), so a named server lock serializes redeems instead.
// Repeating a redeem with the same code, username and password (a double-clicked
// form) returns the account it already made.
//
// It ignores the caller's cancellation: a browser giving up between creating the
// account and spending the code would otherwise leave an account with the code
// still open. A fixed timeout bounds it instead.
func (s *Store) Redeem(ctx context.Context, codeHash, username, md5Pass string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	conn, unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()

	var friendID int64
	var used sql.NullTime
	var revoked bool
	var usedAcct sql.NullInt64
	var usedName sql.NullString
	err = s.portal.QueryRowContext(ctx, "SELECT id, used_at, revoked, account_id, username FROM friends WHERE code_hash=?", codeHash).
		Scan(&friendID, &used, &revoked, &usedAcct, &usedName)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrBadCode
	case err != nil:
		return 0, err
	case used.Valid:
		if usedAcct.Valid && usedName.String == username {
			var one int
			err := conn.QueryRowContext(ctx, "SELECT 1 FROM login WHERE account_id=? AND userid=? AND user_pass=? LIMIT 1",
				usedAcct.Int64, username, md5Pass).Scan(&one)
			if err == nil {
				return usedAcct.Int64, nil
			} else if !errors.Is(err, sql.ErrNoRows) {
				return 0, err
			}
		}
		return 0, ErrBadCode
	case revoked:
		return 0, ErrRevoked
	}

	var exists int
	err = conn.QueryRowContext(ctx, "SELECT 1 FROM login WHERE userid=? LIMIT 1", username).Scan(&exists)
	if err == nil {
		return 0, ErrUsernameTaken
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	r, err := conn.ExecContext(ctx, "INSERT INTO login (userid, user_pass, sex, group_id) VALUES (?, ?, 'M', 0)", username, md5Pass)
	if err != nil {
		return 0, err
	}
	acct, err := r.LastInsertId()
	if err != nil {
		return 0, err
	}
	u, err := s.portal.ExecContext(ctx,
		"UPDATE friends SET used_at=NOW(), account_id=?, username=? WHERE id=? AND used_at IS NULL", acct, username, friendID)
	if n, _ := rowsAffected(u, err); n != 1 {
		// The account exists but the code is unspent. The portal's game-DB user can't
		// delete logins, so leave a loud log line for the admin to clean up by hand.
		slog.Error("redeem: account created but invite not marked used; remove it by hand",
			"account", acct, "username", username, "friend", friendID, "err", err)
		if err != nil {
			return 0, err
		}
		return 0, ErrBadCode
	}
	return acct, nil
}

func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

// CheckAdmin reports whether username/md5Pass is an rAthena account in GM group 99.
func (s *Store) CheckAdmin(ctx context.Context, username, md5Pass string) (bool, error) {
	var one int
	err := s.game.QueryRowContext(ctx,
		"SELECT 1 FROM login WHERE userid=? AND user_pass=? AND group_id=99 AND state=0 LIMIT 1", username, md5Pass).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
