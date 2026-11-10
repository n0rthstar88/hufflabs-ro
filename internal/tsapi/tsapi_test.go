package tsapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const listJSON = `[
 {"id":"i1","deviceId":12345,"inviteUrl":"https://login.tailscale.com/admin/invite/aaa","accepted":false},
 {"id":"i2","deviceId":12345,"inviteUrl":"https://login.tailscale.com/admin/invite/bbb","accepted":true,
  "acceptedBy":{"id":33223,"loginName":"friend@example.com"}}
]`

// fake serves the OAuth token endpoint plus whatever handler the test supplies.
func fake(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		h(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, "cid", "csecret", "nDEV123")
}

func TestListInvites(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/device/nDEV123/device-invites" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, listJSON)
	})
	m, err := c.ListInvites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["i1"].Accepted || !m["i2"].Accepted || m["i2"].URL != "https://login.tailscale.com/admin/invite/bbb" {
		t.Errorf("invites = %+v", m)
	}
}

func TestFindByURL(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, listJSON) })
	inv, err := c.FindByURL(context.Background(), " https://login.tailscale.com/admin/invite/bbb ")
	if err != nil {
		t.Fatal(err)
	}
	if inv.ID != "i2" || !inv.Accepted {
		t.Errorf("inv = %+v", inv)
	}
	if _, err := c.FindByURL(context.Background(), "https://login.tailscale.com/admin/invite/zzz"); err != ErrNotFound {
		t.Errorf("unknown url err = %v, want ErrNotFound", err)
	}
}

func TestDeleteInvite(t *testing.T) {
	called := false
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v2/device-invites/i1" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	})
	if err := c.DeleteInvite(context.Background(), "i1"); err != nil || !called {
		t.Errorf("err=%v called=%v", err, called)
	}
}

func TestHTTPError(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"message":"insufficient scope"}`)
	})
	_, err := c.ListInvites(context.Background())
	if err == nil || !strings.Contains(err.Error(), "insufficient scope") || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v", err)
	}
}
