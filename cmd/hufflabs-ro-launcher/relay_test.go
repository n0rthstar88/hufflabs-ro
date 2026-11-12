package main

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestWebTargetReadsClientInfo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	xml := "<?xml version=\"1.0\" encoding=\"euc-kr\" ?>\r\n<clientinfo>\r\n<connection>\r\n\t<address>100.64.1.2</address>\r\n\t<port>6900</port>\r\n</connection>\r\n</clientinfo>\r\n"
	os.WriteFile(filepath.Join(dir, "data", "sclientinfo.xml"), []byte(xml), 0o644)
	got, err := webTarget(dir)
	if err != nil || got != "100.64.1.2:8888" {
		t.Fatalf("webTarget = %q, %v; want 100.64.1.2:8888", got, err)
	}
}

func TestWebTargetFallsBackToClientInfo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	os.WriteFile(filepath.Join(dir, "data", "clientinfo.xml"), []byte("<address> host.example </address>"), 0o644)
	got, err := webTarget(dir)
	if err != nil || got != "host.example:8888" {
		t.Fatalf("webTarget = %q, %v", got, err)
	}
}

func TestWebTargetRejectsOddAddress(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	os.WriteFile(filepath.Join(dir, "data", "sclientinfo.xml"), []byte("<address>a b;c</address>"), 0o644)
	if got, err := webTarget(dir); err == nil {
		t.Fatalf("webTarget = %q, want error", got)
	}
}

func TestWebTargetMissing(t *testing.T) {
	if _, err := webTarget(t.TempDir()); err == nil {
		t.Fatal("want error when no clientinfo exists")
	}
}

func TestRelayCopiesBothWays(t *testing.T) {
	up, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	go func() {
		c, err := up.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4)
		io.ReadFull(c, buf)
		c.Write(append([]byte("re:"), buf...))
	}()

	stop, addr, err := startRelay("127.0.0.1:0", up.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("ping"))
	got, _ := io.ReadAll(c)
	if string(got) != "re:ping" {
		t.Fatalf("got %q", got)
	}
}

func TestRelayStopClosesListener(t *testing.T) {
	stop, addr, err := startRelay("127.0.0.1:0", "127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("listener still open after stop")
	}
}

func TestRelayPortBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, _, err := startRelay(l.Addr().String(), "127.0.0.1:1"); err == nil {
		t.Fatal("want error when the port is taken")
	}
}
