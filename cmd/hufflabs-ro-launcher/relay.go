package main

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// The client loads its account settings (hotkeys, window layout) and guild
// emblems from a web service it always looks for at 127.0.0.1:8888 on the
// player's own PC. rAthena's web-server runs next to the game server, so while
// the game runs the launcher forwards that local port to the server.
const (
	webListen = "127.0.0.1:8888"
	webPort   = "8888"
)

var addressRE = regexp.MustCompile(`<address>\s*([^<]*?)\s*</address>`)
var hostRE = regexp.MustCompile(`^[A-Za-z0-9.\-]+$`)

// webTarget returns the game server's host (from the client's own clientinfo
// file, so it always matches the server the client logs into) with port 8888.
func webTarget(dir string) (string, error) {
	for _, name := range []string{"sclientinfo.xml", "clientinfo.xml"} {
		b, err := os.ReadFile(filepath.Join(dir, "data", name))
		if err != nil {
			continue
		}
		m := addressRE.FindSubmatch(b)
		if m == nil {
			continue
		}
		host := string(m[1])
		if !hostRE.MatchString(host) {
			return "", errors.New("unexpected server address in data/" + name)
		}
		return net.JoinHostPort(host, webPort), nil
	}
	return "", errors.New("no server address in data/sclientinfo.xml or data/clientinfo.xml")
}

// startRelay listens on listen and copies every connection to target. stop
// closes the listener and any open connections. addr is the real listen
// address (useful when listen ends in :0).
func startRelay(listen, target string) (stop func(), addr string, err error) {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, "", err
	}
	var mu sync.Mutex
	conns := map[net.Conn]struct{}{}
	track := func(c net.Conn, add bool) {
		mu.Lock()
		defer mu.Unlock()
		if add {
			conns[c] = struct{}{}
		} else {
			delete(conns, c)
		}
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				up, err := net.DialTimeout("tcp", target, 10*time.Second)
				if err != nil {
					slog.Warn("web relay dial", "target", target, "err", err)
					return
				}
				defer up.Close()
				track(c, true)
				track(up, true)
				defer track(c, false)
				defer track(up, false)
				done := make(chan struct{}, 2)
				go func() { io.Copy(up, c); closeWrite(up); done <- struct{}{} }()
				go func() { io.Copy(c, up); closeWrite(c); done <- struct{}{} }()
				<-done
				<-done
			}()
		}
	}()
	return func() {
		l.Close()
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	}, l.Addr().String(), nil
}

func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		tc.CloseWrite()
	}
}
