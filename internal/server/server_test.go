package server

import (
	"context"
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

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Bind: "127.0.0.1",
			Port: 6380,
		},
		HTTP: config.HTTPConfig{
			Enabled: false,
		},
	}

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected server")
	}
	if s.store == nil {
		t.Error("expected store")
	}
	if s.router == nil {
		t.Error("expected router")
	}
}

func TestNewServerWithHTTP(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Bind: "127.0.0.1",
			Port: 6381,
		},
		HTTP: config.HTTPConfig{
			Enabled:  true,
			Port:     8081,
			Password: "",
		},
	}

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.httpServer == nil {
		t.Error("expected HTTP server")
	}
}

func TestServerStore(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 6382},
		HTTP:   config.HTTPConfig{Enabled: false},
	}

	s, _ := New(cfg)
	if s.Store() == nil {
		t.Error("expected store")
	}
}

func TestServerStartStop(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 0},
		HTTP:   config.HTTPConfig{Enabled: false},
	}

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = s.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error starting server: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = s.Stop(stopCtx)
	if err != nil {
		t.Fatalf("unexpected error stopping server: %v", err)
	}
}

func TestHTTPConfigDefaults(t *testing.T) {
	cfg := HTTPConfig{}
	if cfg.Enabled != false {
		t.Error("expected Enabled to be false (Go zero value, yaml default is only applied via yaml parsing)")
	}
	if cfg.Port != 0 {
		t.Error("expected Port default 0")
	}
}

func newTestHTTPServer() *HTTPServer {
	s := store.NewStore()
	cfg := &HTTPConfig{Enabled: true, Port: 8080}
	router := command.NewRouter()
	return NewHTTPServer(s, router, cfg)
}

func TestNewHTTPServer(t *testing.T) {
	h := newTestHTTPServer()
	if h == nil {
		t.Fatal("expected HTTP server")
	}
	if h.store == nil {
		t.Error("expected store")
	}
	if h.server == nil {
		t.Error("expected http.Server")
	}
}

func TestHTTPServerHealth(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	h.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("expected 'ok' in response, got %s", w.Body.String())
	}
}

func TestHTTPServerInfo(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/info", nil)
	w := httptest.NewRecorder()

	h.handleInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerMetrics(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/metrics", nil)
	w := httptest.NewRecorder()

	h.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "cachestorm_keys_total") {
		t.Errorf("expected metrics in response, got %s", w.Body.String())
	}
}

func TestHTTPServerKeysGET(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	h.store.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	req := httptest.NewRequest("GET", "/api/keys", nil)
	w := httptest.NewRecorder()

	h.handleKeys(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerKeysGETWithPattern(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("prefix_key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	h.store.Set("other_key", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})

	req := httptest.NewRequest("GET", "/api/keys?pattern=prefix*", nil)
	w := httptest.NewRecorder()

	h.handleKeys(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerKeysPOST(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"testkey","value":"testvalue","type":"string"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerKeyGET(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("mykey", &store.StringValue{Data: []byte("myvalue")}, store.SetOptions{})

	req := httptest.NewRequest("GET", "/api/key/mykey", nil)
	w := httptest.NewRecorder()

	h.handleKey(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerKeyGETNotFound(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/key/nonexistent", nil)
	w := httptest.NewRecorder()

	h.handleKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHTTPServerKeyDELETE(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("mykey", &store.StringValue{Data: []byte("myvalue")}, store.SetOptions{})

	req := httptest.NewRequest("DELETE", "/api/key/mykey", nil)
	w := httptest.NewRecorder()

	h.handleKey(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerTags(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/tags", nil)
	w := httptest.NewRecorder()

	h.handleTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerTag(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/tag/mytag", nil)
	w := httptest.NewRecorder()

	h.handleTag(w, req)
}

func TestHTTPServerInvalidate(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("POST", "/api/invalidate/mytag", nil)
	w := httptest.NewRecorder()

	h.handleInvalidate(w, req)
}

func TestHTTPServerNamespacesGET(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/namespaces", nil)
	w := httptest.NewRecorder()

	h.handleNamespaces(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerNamespacesPOST(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"name":"mynamespace"}`
	req := httptest.NewRequest("POST", "/api/namespaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleNamespaces(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerCluster(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/cluster", nil)
	w := httptest.NewRecorder()

	h.handleCluster(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerClusterJoin(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"host":"127.0.0.1","port":6379}`
	req := httptest.NewRequest("POST", "/api/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleClusterJoin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecute(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"command":"PING","args":[]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteGET(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("mykey", &store.StringValue{Data: []byte("myvalue")}, store.SetOptions{})

	body := `{"command":"GET","args":["mykey"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteSET(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"command":"SET","args":["key1","value1"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteDEL(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	body := `{"command":"DEL","args":["key1"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteKEYS(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	body := `{"command":"KEYS","args":["*"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteDBSIZE(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	body := `{"command":"DBSIZE","args":[]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteINFO(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"command":"INFO","args":[]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteFLUSHDB(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"command":"FLUSHDB","args":[]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteTTL(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	body := `{"command":"TTL","args":["key1"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteTYPE(t *testing.T) {
	h := newTestHTTPServer()
	h.store.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})

	body := `{"command":"TYPE","args":["key1"]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerExecuteUnknown(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"command":"UNKNOWN","args":[]}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerSlowlog(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/slowlog", nil)
	w := httptest.NewRecorder()

	h.handleSlowlog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerStats(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()

	h.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerLoginNoAuth(t *testing.T) {
	h := newTestHTTPServer()

	body := `{}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerLoginSuccess(t *testing.T) {
	s := store.NewStore()
	cfg := &HTTPConfig{Enabled: true, Port: 8080, Password: "secret"}
	router := command.NewRouter()
	h := NewHTTPServer(s, router, cfg)

	body := `{"password":"secret"}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerLoginFail(t *testing.T) {
	s := store.NewStore()
	cfg := &HTTPConfig{Enabled: true, Port: 8080, Password: "secret"}
	router := command.NewRouter()
	h := NewHTTPServer(s, router, cfg)

	body := `{"password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHTTPServerStaticRoot(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.handleStatic(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHTTPServerStaticNotFound(t *testing.T) {
	h := newTestHTTPServer()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	h.handleStatic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHTTPAuthMiddlewareNoPassword(t *testing.T) {
	h := newTestHTTPServer()

	called := false
	handler := h.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("expected handler to be called when no password configured")
	}
}

func TestHTTPAuthMiddlewareWithToken(t *testing.T) {
	s := store.NewStore()
	cfg := &HTTPConfig{Enabled: true, Port: 8080, Password: "secret"}
	router := command.NewRouter()
	h := NewHTTPServer(s, router, cfg)

	called := false
	handler := h.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("expected handler to be called with valid token")
	}
}

func TestHTTPAuthMiddlewareUnauthorized(t *testing.T) {
	s := store.NewStore()
	cfg := &HTTPConfig{Enabled: true, Port: 8080, Password: "secret"}
	router := command.NewRouter()
	h := NewHTTPServer(s, router, cfg)

	called := false
	handler := h.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("expected handler not to be called without token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestHTTPCORSMiddleware(t *testing.T) {
	h := newTestHTTPServer()

	handler := h.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header")
	}
}

func TestHTTPWriteJSON(t *testing.T) {
	h := newTestHTTPServer()

	w := httptest.NewRecorder()
	h.writeJSON(w, http.StatusOK, map[string]string{"test": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type application/json")
	}
}

func TestHTTPWriteError(t *testing.T) {
	h := newTestHTTPServer()

	w := httptest.NewRecorder()
	h.writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		expect  bool
	}{
		{"key1", "*", true},
		{"key1", "key1", true},
		{"key1", "key2", false},
		{"key1", "key*", true},
		{"key1", "*1", true},
		{"key1", "*key*", true},
		{"key1", "*foo*", false},
	}

	for _, tt := range tests {
		result := matchPattern(tt.s, tt.pattern)
		if result != tt.expect {
			t.Errorf("matchPattern(%s, %s) = %v, expected %v", tt.s, tt.pattern, result, tt.expect)
		}
	}
}

func TestExecuteCommandErrors(t *testing.T) {
	h := newTestHTTPServer()

	tests := []struct {
		cmd  string
		args [][]byte
	}{
		{"GET", [][]byte{}},
		{"SET", [][]byte{}},
		{"DEL", [][]byte{}},
		{"TTL", [][]byte{}},
		{"TYPE", [][]byte{}},
	}

	for _, tt := range tests {
		result := h.executeCommand(tt.cmd, tt.args)
		if result == nil {
			t.Errorf("expected error message for %s", tt.cmd)
		}
	}
}

func TestHTTPServerStopNil(t *testing.T) {
	h := newTestHTTPServer()

	err := h.Stop()
	if err != nil {
		t.Errorf("unexpected error stopping server: %v", err)
	}
}

func TestNewConnection(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(1, server, s, router)
	if conn.ID != 1 {
		t.Errorf("expected ID 1, got %d", conn.ID)
	}
	if conn.namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", conn.namespace)
	}
}

func TestConnectionRemoteAddr(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(1, server, s, router)
	addr := conn.RemoteAddr()
	if addr == "" {
		t.Error("expected remote address")
	}
}

func TestConnectionClose(t *testing.T) {
	s := store.NewStore()
	router := command.NewRouter()

	client, server := net.Pipe()
	defer client.Close()

	conn := NewConnection(1, server, s, router)
	conn.Close()
}

func TestServerStopWithoutStart(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 16380},
		HTTP:   config.HTTPConfig{Enabled: false},
	}

	s, _ := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := s.Stop(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestServerWithHTTPEnabled(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1", Port: 0},
		HTTP: config.HTTPConfig{
			Enabled:  true,
			Port:     0,
			Password: "",
		},
	}

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = s.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error starting server: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = s.Stop(stopCtx)
	if err != nil {
		t.Fatalf("unexpected error stopping server: %v", err)
	}
}

func TestHTTPServerKeysInvalidJSON(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":invalid`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHTTPServerNamespacesInvalidJSON(t *testing.T) {
	h := newTestHTTPServer()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/namespaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleNamespaces(w, req)
}

func TestHTTPServerClusterJoinInvalidJSON(t *testing.T) {
	h := newTestHTTPServer()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleClusterJoin(w, req)
}

func TestHTTPServerExecuteInvalidJSON(t *testing.T) {
	h := newTestHTTPServer()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHTTPServerLoginInvalidJSON(t *testing.T) {
	h := newTestHTTPServer()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleLogin(w, req)
}

func TestHTTPServerKeysWithTTL(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"ttlkey","value":"value","type":"string","ttl":"1h"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)
}

func TestHTTPServerKeysWithTypeList(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"listkey","value":"a,b,c","type":"list"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)
}

func TestHTTPServerKeysWithTypeSet(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"setkey","value":"a,b,c","type":"set"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)
}

func TestHTTPServerKeysWithTypeHash(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"hashkey","value":"field1=value1,field2=value2","type":"hash"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)
}

func TestHTTPServerKeysWithTags(t *testing.T) {
	h := newTestHTTPServer()

	body := `{"key":"taggedkey","value":"value","type":"string","tags":"tag1,tag2"}`
	req := httptest.NewRequest("POST", "/api/keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleKeys(w, req)
}

func TestHTTPServerHandleMethods(t *testing.T) {
	h := newTestHTTPServer()

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/health", ""},
		{"GET", "/api/info", ""},
		{"GET", "/api/metrics", ""},
		{"GET", "/api/keys", ""},
		{"GET", "/api/tags", ""},
		{"GET", "/api/namespaces", ""},
		{"GET", "/api/cluster", ""},
		{"GET", "/api/slowlog", ""},
		{"GET", "/api/stats", ""},
	}

	for _, tt := range tests {
		var req *http.Request
		if tt.body != "" {
			req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		} else {
			req = httptest.NewRequest(tt.method, tt.path, nil)
		}
		w := httptest.NewRecorder()

		switch tt.path {
		case "/api/health":
			h.handleHealth(w, req)
		case "/api/info":
			h.handleInfo(w, req)
		case "/api/metrics":
			h.handleMetrics(w, req)
		case "/api/keys":
			h.handleKeys(w, req)
		case "/api/tags":
			h.handleTags(w, req)
		case "/api/namespaces":
			h.handleNamespaces(w, req)
		case "/api/cluster":
			h.handleCluster(w, req)
		case "/api/slowlog":
			h.handleSlowlog(w, req)
		case "/api/stats":
			h.handleStats(w, req)
		}
	}
}
