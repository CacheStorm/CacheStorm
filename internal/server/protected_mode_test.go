package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestIsLoopbackAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:5000":   true,
		"[::1]:5000":       true,
		"127.10.0.3:1":     true,
		"10.0.0.5:5000":    false,
		"203.0.113.9:6380": false,
		"[2001:db8::1]:80": false,
		"pipe":             true,
		"@":                true,
	} {
		if got := isLoopbackAddr(addr); got != want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestHTTPProtectedModeRefusesRemoteAnonymousRequests(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remote    string
		password  string
		protected bool
		want      int
	}{
		{"remote, protected", "203.0.113.9:1234", "", true, http.StatusForbidden},
		{"loopback, protected", "127.0.0.1:1234", "", true, http.StatusOK},
		{"remote, protection off", "203.0.113.9:1234", "", false, http.StatusOK},
		{"remote, password set", "203.0.113.9:1234", "pw", true, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHTTPServer(store.NewStore(), command.NewRouter(), &HTTPConfig{Password: tc.password, Protected: tc.protected})
			defer h.cancel()
			req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
			req.RemoteAddr = tc.remote
			if tc.password != "" {
				req.Header.Set("Authorization", "Bearer "+tc.password)
			}
			rec := httptest.NewRecorder()
			h.authMiddleware(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// nonLoopbackIPv4 returns an address of this host that is not loopback, so a
// connection to it arrives from a non-loopback source.
func nonLoopbackIPv4(t *testing.T) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skip(err)
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	t.Skip("no non-loopback IPv4 address on this host")
	return ""
}

func TestRESPProtectedMode(t *testing.T) {
	ip := nonLoopbackIPv4(t)
	for _, tc := range []struct {
		name      string
		password  string
		protected bool
		denied    bool
	}{
		{"protected, no password", "", true, true},
		{"protection off", "", false, false},
		{"password set", "pw", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Server.Bind = "0.0.0.0"
			cfg.Server.Port = 0
			cfg.Server.RequirePass = tc.password
			cfg.Server.ProtectedMode = tc.protected
			cfg.HTTP.Enabled = false
			cfg.Plugins.Metrics.Enabled = false
			s, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer s.Stop(context.Background())
			_, port, _ := net.SplitHostPort(s.listener.Addr().String())

			c, err := net.Dial("tcp", net.JoinHostPort(ip, port))
			if err != nil {
				t.Skip(err)
			}
			defer c.Close()
			_, _ = c.Write([]byte("*1\r\n$4\r\nPING\r\n"))
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 64)
			n, _ := io.ReadAtLeast(c, buf, 1)
			got := string(buf[:n])
			if tc.denied != strings.HasPrefix(got, "-DENIED") {
				t.Fatalf("reply %q, denied=%v", got, tc.denied)
			}

			// Loopback clients are always served.
			lc, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", port))
			if err != nil {
				t.Fatal(err)
			}
			defer lc.Close()
			_, _ = lc.Write([]byte("*1\r\n$4\r\nPING\r\n"))
			_ = lc.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _ = io.ReadAtLeast(lc, buf, 1)
			if strings.HasPrefix(string(buf[:n]), "-DENIED") {
				t.Fatalf("loopback client denied: %q", buf[:n])
			}
		})
	}
}
