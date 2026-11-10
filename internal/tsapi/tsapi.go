// Package tsapi reads and deletes Tailscale share invites for the game server.
//
// Tailscale does not let OAuth clients create device invites (a share belongs to a
// user), so the admin creates each share link in the admin console and the portal only
// looks it up, tracks acceptance and deletes it on revoke. The OAuth client needs
// just the `device_invites` scope.
package tsapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

var ErrNotFound = errors.New("share link not found for this machine")

type Invite struct {
	ID       string
	URL      string
	Accepted bool
}

type Client struct {
	base     string
	deviceID string
	hc       *http.Client
}

func New(baseURL, clientID, clientSecret, deviceID string) *Client {
	base := strings.TrimRight(baseURL, "/")
	cc := clientcredentials.Config{ClientID: clientID, ClientSecret: clientSecret, TokenURL: base + "/api/v2/oauth/token"}
	hc := cc.Client(context.Background())
	hc.Timeout = 15 * time.Second
	return &Client{base: base, deviceID: deviceID, hc: hc}
}

func (c *Client) do(ctx context.Context, method, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		var e struct{ Message string }
		json.Unmarshal(body, &e)
		if e.Message == "" {
			e.Message = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("tailscale api %s %s: %d: %s", method, path, resp.StatusCode, e.Message)
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

// ListInvites returns the machine's share invites keyed by invite ID.
func (c *Client) ListInvites(ctx context.Context) (map[string]Invite, error) {
	var raw []struct {
		ID        string `json:"id"`
		InviteURL string `json:"inviteUrl"`
		Accepted  bool   `json:"accepted"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v2/device/"+url.PathEscape(c.deviceID)+"/device-invites", &raw); err != nil {
		return nil, err
	}
	m := make(map[string]Invite, len(raw))
	for _, r := range raw {
		m[r.ID] = Invite{ID: r.ID, URL: r.InviteURL, Accepted: r.Accepted}
	}
	return m, nil
}

// FindByURL finds the invite whose link the admin pasted.
func (c *Client) FindByURL(ctx context.Context, link string) (Invite, error) {
	link = strings.TrimSpace(link)
	m, err := c.ListInvites(ctx)
	if err != nil {
		return Invite{}, err
	}
	for _, inv := range m {
		if inv.URL == link {
			return inv, nil
		}
	}
	return Invite{}, ErrNotFound
}

func (c *Client) DeleteInvite(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v2/device-invites/"+url.PathEscape(id), nil)
}
