package main

import (
	"net"
	"net/http"
	"testing"
	"time"

	"jk-api/config"

	"github.com/gofiber/fiber/v2"
)

// TestClientIPBehindProxy pins what ends up in the ip column of login_logs,
// activity_logs and user_consents. It runs over a real loopback socket rather
// than app.Test, because the whole question is what the peer address is: only a
// real connection makes the peer 127.0.0.1, i.e. a trusted proxy.
func TestClientIPBehindProxy(t *testing.T) {
	cfg := &config.Config{
		AppName:        "test",
		TrustedProxies: "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16",
	}
	app := fiber.New(fiberConfig(cfg))
	app.Get("/ip", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	defer func() { _ = app.Shutdown() }()

	url := "http://" + ln.Addr().String() + "/ip"
	client := &http.Client{Timeout: 5 * time.Second}

	get := func(t *testing.T, xff string) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if xff != "" {
			req.Header.Set(fiber.HeaderXForwardedFor, xff)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer res.Body.Close()
		buf := make([]byte, 64)
		n, _ := res.Body.Read(buf)
		return string(buf[:n])
	}

	// nginx appends with $proxy_add_x_forwarded_for, so the client leads the
	// list and the docker gateway trails it. Only the client may be stored —
	// this is the case that returned the whole raw string before EnableIPValidation.
	if got := get(t, "203.0.113.9, 172.18.0.1"); got != "203.0.113.9" {
		t.Errorf("chained X-Forwarded-For: got %q, want %q", got, "203.0.113.9")
	}

	if got := get(t, "203.0.113.9"); got != "203.0.113.9" {
		t.Errorf("single X-Forwarded-For: got %q, want %q", got, "203.0.113.9")
	}

	// No header (a direct hit on the container's port) must still record
	// something real rather than an empty string.
	if got := get(t, ""); got != "127.0.0.1" {
		t.Errorf("no X-Forwarded-For: got %q, want %q", got, "127.0.0.1")
	}
}

// TestUntrustedPeerCannotForgeIP is the security half: with the proxy list
// empty, nothing is trusted, so a header a client sends itself is ignored.
func TestUntrustedPeerCannotForgeIP(t *testing.T) {
	cfg := &config.Config{AppName: "test", TrustedProxies: ""}
	app := fiber.New(fiberConfig(cfg))
	app.Get("/ip", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	defer func() { _ = app.Shutdown() }()

	req, err := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+"/ip", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(fiber.HeaderXForwardedFor, "1.2.3.4")
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()
	buf := make([]byte, 64)
	n, _ := res.Body.Read(buf)
	if got := string(buf[:n]); got != "127.0.0.1" {
		t.Errorf("untrusted peer: got %q, want the socket address %q", got, "127.0.0.1")
	}
}
