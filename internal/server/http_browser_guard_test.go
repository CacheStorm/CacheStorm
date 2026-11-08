package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A web page can make a visitor's browser POST to the loopback API without a
// preflight, or rebind its own host name to 127.0.0.1; neither may run a
// command. curl-style clients and the API's own dashboard still can.
func TestHTTPRefusesBrowserCrossOriginAndRebinding(t *testing.T) {
	st := store.NewStore()
	router := command.NewRouter()
	command.RegisterStringCommands(router)
	h := NewHTTPServer(st, router, &HTTPConfig{})
	t.Cleanup(h.cancel)
	run := func(host, origin, key string) int {
		body := `{"command":"SET","args":["` + key + `","v"]}`
		r := httptest.NewRequest("POST", "http://"+host+"/api/execute", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:50000"
		r.Header.Set("Content-Type", "text/plain")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.server.Handler.ServeHTTP(w, r)
		return w.Code
	}
	cases := []struct {
		name, host, origin, key string
		want                    int
	}{
		{"curl, no origin", "127.0.0.1:8080", "", "a", 200},
		{"own dashboard", "127.0.0.1:8080", "http://127.0.0.1:8080", "b", 200},
		{"localhost name", "localhost:8080", "", "c", 200},
		{"cross-origin page", "127.0.0.1:8080", "https://evil.example", "d", 403},
		{"null origin", "127.0.0.1:8080", "null", "e", 403},
		{"rebound host name", "evil.example:8080", "http://evil.example:8080", "f", 403},
	}
	for _, c := range cases {
		if got := run(c.host, c.origin, c.key); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
		_, exists := st.Get(c.key)
		if exists != (c.want == 200) {
			t.Errorf("%s: key written = %v", c.name, exists)
		}
	}

	h.config.CORSOrigin = "https://app.example"
	if got := run("127.0.0.1:8080", "https://app.example", "g"); got != 200 {
		t.Errorf("configured cors_origin: status %d", got)
	}
}
