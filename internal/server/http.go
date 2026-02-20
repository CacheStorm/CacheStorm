package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

type HTTPConfig struct {
	Enabled  bool   `yaml:"enabled" default:"true"`
	Port     int    `yaml:"port" default:"8080"`
	Password string `yaml:"password"`
}

type HTTPServer struct {
	store   *store.Store
	router  *command.Router
	server  *http.Server
	started time.Time
	config  *HTTPConfig
}

func NewHTTPServer(s *store.Store, router *command.Router, cfg *HTTPConfig) *HTTPServer {
	h := &HTTPServer{
		store:   s,
		router:  router,
		started: time.Now(),
		config:  cfg,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", h.handleHealth)
	mux.HandleFunc("/api/info", h.authMiddleware(h.handleInfo))
	mux.HandleFunc("/api/metrics", h.handleMetrics)
	mux.HandleFunc("/api/keys", h.authMiddleware(h.handleKeys))
	mux.HandleFunc("/api/key/", h.authMiddleware(h.handleKey))
	mux.HandleFunc("/api/tags", h.authMiddleware(h.handleTags))
	mux.HandleFunc("/api/tag/", h.authMiddleware(h.handleTag))
	mux.HandleFunc("/api/invalidate/", h.authMiddleware(h.handleInvalidate))
	mux.HandleFunc("/api/namespaces", h.authMiddleware(h.handleNamespaces))
	mux.HandleFunc("/api/namespace/", h.authMiddleware(h.handleNamespace))
	mux.HandleFunc("/api/cluster", h.authMiddleware(h.handleCluster))
	mux.HandleFunc("/api/cluster/join", h.authMiddleware(h.handleClusterJoin))
	mux.HandleFunc("/api/execute", h.authMiddleware(h.handleExecute))
	mux.HandleFunc("/api/slowlog", h.authMiddleware(h.handleSlowlog))
	mux.HandleFunc("/api/stats", h.authMiddleware(h.handleStats))
	mux.HandleFunc("/api/login", h.handleLogin)

	mux.HandleFunc("/", h.handleStatic)

	h.server = &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           h.corsMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	return h
}

func (h *HTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *HTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.config.Password == "" {
			next(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			cookie, err := r.Cookie("auth_token")
			if err == nil {
				token = cookie.Value
			}
		}

		token = strings.TrimPrefix(token, "Bearer ")

		if subtle.ConstantTimeCompare([]byte(token), []byte(h.config.Password)) != 1 {
			h.writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

func (h *HTTPServer) Start() error {
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Stop() error {
	return h.server.Close()
}

func (h *HTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *HTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]interface{}{
		"error":  true,
		"status": status,
		"reason": message,
	})
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(h.started).String(),
	})
}

func (h *HTTPServer) handleInfo(w http.ResponseWriter, _ *http.Request) {
	info := map[string]interface{}{
		"server": map[string]interface{}{
			"version":    "0.1.0",
			"uptime":     time.Since(h.started).String(),
			"keys":       h.store.KeyCount(),
			"memory":     h.store.MemUsage(),
			"started_at": h.started,
		},
		"store": map[string]interface{}{
			"shards":   store.NumShards,
			"keys":     h.store.KeyCount(),
			"mem_used": h.store.MemUsage(),
		},
	}
	h.writeJSON(w, http.StatusOK, info)
}

func (h *HTTPServer) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	keys := h.store.KeyCount()
	mem := h.store.MemUsage()

	metrics := fmt.Sprintf(`# HELP cachestorm_keys_total Total number of keys
# TYPE cachestorm_keys_total gauge
cachestorm_keys_total %d
# HELP cachestorm_memory_bytes Memory usage in bytes
# TYPE cachestorm_memory_bytes gauge
cachestorm_memory_bytes %d
# HELP cachestorm_uptime_seconds Server uptime in seconds
# TYPE cachestorm_uptime_seconds gauge
cachestorm_uptime_seconds %.0f
`, keys, mem, time.Since(h.started).Seconds())

	w.Write([]byte(metrics))
}

func (h *HTTPServer) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		pattern := r.URL.Query().Get("pattern")
		if pattern == "" {
			pattern = "*"
		}

		keys := h.store.Keys()

		if pattern != "*" {
			filtered := make([]string, 0)
			for _, k := range keys {
				if matchPattern(k, pattern) {
					filtered = append(filtered, k)
				}
			}
			keys = filtered
		}

		keyData := make([]map[string]interface{}, 0)
		for _, k := range keys {
			entry, exists := h.store.Get(k)
			if exists {
				ttl := h.store.TTL(k)
				ttlStr := "-1"
				if ttl > 0 {
					ttlStr = ttl.String()
				} else if ttl == -2 {
					ttlStr = "expired"
				}

				keyData = append(keyData, map[string]interface{}{
					"key":  k,
					"type": entry.Value.Type().String(),
					"ttl":  ttlStr,
					"size": entry.Value.SizeOf(),
					"tags": entry.Tags,
				})
			}
		}

		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"count": len(keyData),
			"keys":  keyData,
		})

	case "POST":
		var req struct {
			Key   string        `json:"key"`
			Value string        `json:"value"`
			TTL   time.Duration `json:"ttl"`
			Type  string        `json:"type"`
			Tags  []string      `json:"tags"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		var value store.Value
		switch strings.ToLower(req.Type) {
		case "string", "":
			value = &store.StringValue{Data: []byte(req.Value)}
		case "list":
			value = &store.ListValue{Elements: [][]byte{[]byte(req.Value)}}
		case "set":
			value = &store.SetValue{Members: map[string]struct{}{req.Value: {}}}
		case "hash":
			value = &store.HashValue{Fields: map[string][]byte{"value": []byte(req.Value)}}
		default:
			value = &store.StringValue{Data: []byte(req.Value)}
		}

		opts := store.SetOptions{TTL: req.TTL, Tags: req.Tags}
		if err := h.store.Set(req.Key, value, opts); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"result": "OK",
			"key":    req.Key,
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/key/")
	if key == "" {
		h.writeError(w, http.StatusBadRequest, "key required")
		return
	}

	switch r.Method {
	case "GET":
		entry, exists := h.store.Get(key)
		if !exists {
			h.writeError(w, http.StatusNotFound, "key not found")
			return
		}

		ttl := h.store.TTL(key)
		ttlStr := "-1"
		if ttl > 0 {
			ttlStr = ttl.String()
		}

		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"key":   key,
			"type":  entry.Value.Type().String(),
			"value": entry.Value.String(),
			"ttl":   ttlStr,
			"tags":  entry.Tags,
			"size":  entry.Value.SizeOf(),
		})

	case "DELETE":
		deleted := h.store.Delete(key)
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"deleted": deleted,
			"key":     key,
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleTags(w http.ResponseWriter, _ *http.Request) {
	tagIndex := h.store.GetTagIndex()
	if tagIndex == nil {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"count": 0,
			"tags":  []interface{}{},
		})
		return
	}

	tags := tagIndex.Tags()
	tagInfo := make([]map[string]interface{}, 0, len(tags))
	for _, tag := range tags {
		tagInfo = append(tagInfo, map[string]interface{}{
			"name":  tag,
			"count": tagIndex.Count(tag),
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"count": len(tags),
		"tags":  tagInfo,
	})
}

func (h *HTTPServer) handleTag(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimPrefix(r.URL.Path, "/api/tag/")
	if tag == "" {
		h.writeError(w, http.StatusBadRequest, "tag required")
		return
	}

	tagIndex := h.store.GetTagIndex()
	if tagIndex == nil {
		h.writeError(w, http.StatusNotFound, "tag not found")
		return
	}

	keys := tagIndex.GetKeys(tag)
	count := tagIndex.Count(tag)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tag":   tag,
		"count": count,
		"keys":  keys,
	})
}

func (h *HTTPServer) handleInvalidate(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimPrefix(r.URL.Path, "/api/invalidate/")
	if tag == "" {
		h.writeError(w, http.StatusBadRequest, "tag required")
		return
	}

	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tagIndex := h.store.GetTagIndex()
	if tagIndex == nil {
		h.writeError(w, http.StatusNotFound, "tag not found")
		return
	}

	keys := tagIndex.Invalidate(tag)

	deletedCount := 0
	for _, key := range keys {
		if h.store.Delete(key) {
			deletedCount++
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tag":          tag,
		"keys_deleted": deletedCount,
		"keys":         keys,
	})
}

func (h *HTTPServer) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	nsMgr := h.store.GetNamespaceManager()
	if nsMgr == nil {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"namespaces": []interface{}{},
		})
		return
	}

	switch r.Method {
	case "GET":
		names := nsMgr.List()
		nsData := make([]map[string]interface{}, 0)
		for _, name := range names {
			ns := nsMgr.Get(name)
			if ns != nil {
				stats, _ := nsMgr.Stats(name)
				nsData = append(nsData, stats)
			}
		}

		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"count":      len(nsData),
			"namespaces": nsData,
		})

	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		nsMgr.GetOrCreate(req.Name)
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"result":    "OK",
			"namespace": req.Name,
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleNamespace(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/namespace/")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "namespace required")
		return
	}

	nsMgr := h.store.GetNamespaceManager()
	if nsMgr == nil {
		h.writeError(w, http.StatusNotFound, "namespace manager not available")
		return
	}

	switch r.Method {
	case "GET":
		stats, err := nsMgr.Stats(name)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, stats)

	case "DELETE":
		if err := nsMgr.Delete(name); err != nil {
			h.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"result":    "OK",
			"namespace": name,
		})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleCluster(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"state":          "ok",
		"slots_assigned": 16384,
		"slots_ok":       16384,
		"known_nodes":    1,
		"size":           1,
		"current_epoch":  1,
		"nodes": []map[string]interface{}{
			{
				"id":        "node-1",
				"addr":      "127.0.0.1:6380",
				"role":      "master",
				"slots":     "0-16383",
				"connected": true,
			},
		},
	})
}

func (h *HTTPServer) handleClusterJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"result":  "OK",
		"message": fmt.Sprintf("Joining cluster at %s:%d", req.Host, req.Port),
	})
}

func (h *HTTPServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	args := make([][]byte, len(req.Args))
	for i, arg := range req.Args {
		args[i] = []byte(arg)
	}

	result := h.executeCommand(strings.ToUpper(req.Command), args)
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"result": result,
	})
}

func (h *HTTPServer) executeCommand(cmd string, args [][]byte) interface{} {
	switch cmd {
	case "GET":
		if len(args) < 1 {
			return "ERROR: wrong number of arguments"
		}
		key := string(args[0])
		entry, exists := h.store.Get(key)
		if !exists {
			return nil
		}
		if sv, ok := entry.Value.(*store.StringValue); ok {
			return string(sv.Data)
		}
		return entry.Value.String()

	case "SET":
		if len(args) < 2 {
			return "ERROR: wrong number of arguments"
		}
		key := string(args[0])
		value := &store.StringValue{Data: args[1]}
		if err := h.store.Set(key, value, store.SetOptions{}); err != nil {
			return "ERROR: " + err.Error()
		}
		return "OK"

	case "DEL":
		if len(args) < 1 {
			return "ERROR: wrong number of arguments"
		}
		count := 0
		for _, arg := range args {
			if h.store.Delete(string(arg)) {
				count++
			}
		}
		return count

	case "KEYS":
		pattern := "*"
		if len(args) > 0 {
			pattern = string(args[0])
		}
		keys := h.store.Keys()
		if pattern != "*" {
			filtered := make([]string, 0)
			for _, k := range keys {
				if matchPattern(k, pattern) {
					filtered = append(filtered, k)
				}
			}
			return filtered
		}
		return keys

	case "DBSIZE":
		return h.store.KeyCount()

	case "PING":
		return "PONG"

	case "INFO":
		return map[string]interface{}{
			"keys":   h.store.KeyCount(),
			"memory": h.store.MemUsage(),
			"uptime": time.Since(h.started).String(),
		}

	case "FLUSHDB":
		h.store.Flush()
		return "OK"

	case "TTL":
		if len(args) < 1 {
			return "ERROR: wrong number of arguments"
		}
		ttl := h.store.TTL(string(args[0]))
		return int(ttl.Milliseconds())

	case "TYPE":
		if len(args) < 1 {
			return "ERROR: wrong number of arguments"
		}
		entry, exists := h.store.Get(string(args[0]))
		if !exists {
			return "none"
		}
		return entry.Value.Type().String()

	default:
		return fmt.Sprintf("ERROR: unknown command '%s'", cmd)
	}
}

func (h *HTTPServer) handleSlowlog(w http.ResponseWriter, r *http.Request) {
	_ = r.URL.Query().Get("count")

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   0,
		"entries": []interface{}{},
		"message": "slowlog requires plugin initialization",
	})
}

func (h *HTTPServer) handleStats(w http.ResponseWriter, _ *http.Request) {
	tagIndex := h.store.GetTagIndex()
	tagCount := 0
	if tagIndex != nil {
		tagCount = len(tagIndex.Tags())
	}

	nsMgr := h.store.GetNamespaceManager()
	nsCount := 0
	if nsMgr != nil {
		nsCount = len(nsMgr.List())
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys":       h.store.KeyCount(),
		"memory":     h.store.MemUsage(),
		"tags":       tagCount,
		"namespaces": nsCount,
		"uptime":     time.Since(h.started).String(),
		"shards":     store.NumShards,
		"started_at": h.started,
	})
}

func (h *HTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if h.config.Password == "" {
		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "no authentication required",
		})
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.config.Password)) == 1 {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    req.Password,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})

		h.writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"token":   req.Password,
		})
		return
	}

	h.writeError(w, http.StatusUnauthorized, "invalid password")
}

func (h *HTTPServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(adminUIHTML))
		return
	}

	h.writeError(w, http.StatusNotFound, "not found")
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(s, middle)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(s, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(s, prefix)
	}

	return s == pattern
}

const adminUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CacheStorm Admin</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script defer src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <style>
        [x-cloak] { display: none !important; }
        .sidebar-item.active { background-color: rgb(30 58 138); }
        pre { white-space: pre-wrap; word-break: break-all; }
    </style>
</head>
<body class="bg-slate-900 text-slate-100 min-h-screen">
    <div x-data="adminApp()" x-init="init()" x-cloak>
        <div x-show="!authenticated && requiresAuth" class="min-h-screen flex items-center justify-center">
            <div class="bg-slate-800 p-8 rounded-xl shadow-2xl w-full max-w-md border border-slate-700">
                <div class="text-center mb-8">
                    <div class="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-blue-500 to-purple-600 rounded-xl mb-4">
                        <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                        </svg>
                    </div>
                    <h1 class="text-2xl font-bold text-white">CacheStorm Admin</h1>
                    <p class="text-slate-400 mt-2">Enter password to continue</p>
                </div>
                <form @submit.prevent="login()">
                    <input type="password" x-model="loginPassword" placeholder="Password"
                        class="w-full px-4 py-3 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 mb-4">
                    <button type="submit" class="w-full py-3 bg-gradient-to-r from-blue-500 to-purple-600 text-white font-semibold rounded-lg hover:from-blue-600 hover:to-purple-700 transition-all">
                        Sign In
                    </button>
                    <p x-show="loginError" x-text="loginError" class="text-red-400 text-center mt-4"></p>
                </form>
            </div>
        </div>

        <div x-show="authenticated || !requiresAuth" class="flex min-h-screen">
            <aside class="w-64 bg-slate-800 border-r border-slate-700 flex flex-col">
                <div class="p-4 border-b border-slate-700">
                    <div class="flex items-center gap-3">
                        <div class="w-10 h-10 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
                            <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                            </svg>
                        </div>
                        <div>
                            <h1 class="font-bold text-white">CacheStorm</h1>
                            <p class="text-xs text-slate-400">Admin Console</p>
                        </div>
                    </div>
                </div>
                <nav class="flex-1 p-4 space-y-1">
                    <button @click="currentView = 'dashboard'" :class="{'active': currentView === 'dashboard'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/></svg>
                        Dashboard
                    </button>
                    <button @click="currentView = 'keys'" :class="{'active': currentView === 'keys'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
                        Keys
                    </button>
                    <button @click="currentView = 'tags'" :class="{'active': currentView === 'tags'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/></svg>
                        Tags
                    </button>
                    <button @click="currentView = 'namespaces'" :class="{'active': currentView === 'namespaces'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/></svg>
                        Namespaces
                    </button>
                    <button @click="currentView = 'cluster'" :class="{'active': currentView === 'cluster'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/></svg>
                        Cluster
                    </button>
                    <button @click="currentView = 'console'" :class="{'active': currentView === 'console'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
                        Console
                    </button>
                    <button @click="currentView = 'slowlog'" :class="{'active': currentView === 'slowlog'}"
                        class="sidebar-item w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-slate-300 hover:bg-slate-700 transition-colors">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                        Slow Log
                    </button>
                </nav>
                <div class="p-4 border-t border-slate-700">
                    <div class="flex items-center gap-3">
                        <div class="w-8 h-8 bg-green-500/20 rounded-full flex items-center justify-center">
                            <div class="w-2 h-2 bg-green-500 rounded-full"></div>
                        </div>
                        <div>
                            <p class="text-sm text-slate-300">Server Status</p>
                            <p class="text-xs text-green-400">Connected</p>
                        </div>
                    </div>
                </div>
            </aside>

            <main class="flex-1 p-6 overflow-auto">
                <div x-show="currentView === 'dashboard'" x-cloak>
                    <h2 class="text-2xl font-bold mb-6">Dashboard</h2>
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <div class="flex items-center justify-between mb-4">
                                <span class="text-slate-400">Total Keys</span>
                                <div class="w-10 h-10 bg-blue-500/20 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
                                </div>
                            </div>
                            <p class="text-3xl font-bold text-white" x-text="stats.keys.toLocaleString()"></p>
                        </div>
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <div class="flex items-center justify-between mb-4">
                                <span class="text-slate-400">Memory Usage</span>
                                <div class="w-10 h-10 bg-purple-500/20 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"/></svg>
                                </div>
                            </div>
                            <p class="text-3xl font-bold text-white" x-text="formatBytes(stats.memory)"></p>
                        </div>
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <div class="flex items-center justify-between mb-4">
                                <span class="text-slate-400">Tags</span>
                                <div class="w-10 h-10 bg-green-500/20 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/></svg>
                                </div>
                            </div>
                            <p class="text-3xl font-bold text-white" x-text="stats.tags.toLocaleString()"></p>
                        </div>
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <div class="flex items-center justify-between mb-4">
                                <span class="text-slate-400">Uptime</span>
                                <div class="w-10 h-10 bg-orange-500/20 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                                </div>
                            </div>
                            <p class="text-3xl font-bold text-white" x-text="stats.uptime"></p>
                        </div>
                    </div>
                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <h3 class="text-lg font-semibold mb-4">Recent Activity</h3>
                            <div class="space-y-3">
                                <template x-for="(activity, i) in recentActivity" :key="i">
                                    <div class="flex items-center gap-3 text-sm">
                                        <div class="w-2 h-2 rounded-full" :class="activity.color"></div>
                                        <span class="text-slate-400" x-text="activity.time"></span>
                                        <span class="text-slate-300" x-text="activity.message"></span>
                                    </div>
                                </template>
                            </div>
                        </div>
                        <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                            <h3 class="text-lg font-semibold mb-4">Top Tags</h3>
                            <div class="space-y-3">
                                <template x-for="tag in topTags" :key="tag.name">
                                    <div class="flex items-center justify-between">
                                        <span class="text-slate-300" x-text="tag.name"></span>
                                        <div class="flex items-center gap-2">
                                            <div class="w-32 bg-slate-700 rounded-full h-2">
                                                <div class="bg-blue-500 h-2 rounded-full" :style="'width: ' + (tag.count / stats.keys * 100 || 0) + '%'"></div>
                                            </div>
                                            <span class="text-slate-400 text-sm w-12 text-right" x-text="tag.count"></span>
                                        </div>
                                    </div>
                                </template>
                            </div>
                        </div>
                    </div>
                </div>

                <div x-show="currentView === 'keys'" x-cloak>
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-2xl font-bold">Keys</h2>
                        <div class="flex gap-3">
                            <input type="text" x-model="keySearch" @input="searchKeys()" placeholder="Search keys..."
                                class="px-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500">
                            <button @click="showAddKeyModal = true" class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
                                Add Key
                            </button>
                        </div>
                    </div>
                    <div class="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
                        <table class="w-full">
                            <thead class="bg-slate-700/50">
                                <tr>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Key</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Type</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">TTL</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Size</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Tags</th>
                                    <th class="px-6 py-3 text-right text-xs font-medium text-slate-400 uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody class="divide-y divide-slate-700">
                                <template x-for="key in filteredKeys" :key="key.key">
                                    <tr class="hover:bg-slate-700/30">
                                        <td class="px-6 py-4 text-sm text-white font-mono" x-text="key.key"></td>
                                        <td class="px-6 py-4">
                                            <span class="px-2 py-1 text-xs rounded-full bg-blue-500/20 text-blue-400" x-text="key.type"></span>
                                        </td>
                                        <td class="px-6 py-4 text-sm text-slate-400" x-text="key.ttl"></td>
                                        <td class="px-6 py-4 text-sm text-slate-400" x-text="formatBytes(key.size)"></td>
                                        <td class="px-6 py-4">
                                            <template x-for="tag in key.tags" :key="tag">
                                                <span class="inline-block px-2 py-1 text-xs rounded-full bg-green-500/20 text-green-400 mr-1" x-text="tag"></span>
                                            </template>
                                        </td>
                                        <td class="px-6 py-4 text-right">
                                            <button @click="viewKey(key)" class="text-blue-400 hover:text-blue-300 mr-3">View</button>
                                            <button @click="deleteKey(key.key)" class="text-red-400 hover:text-red-300">Delete</button>
                                        </td>
                                    </tr>
                                </template>
                            </tbody>
                        </table>
                        <div x-show="filteredKeys.length === 0" class="p-8 text-center text-slate-400">
                            No keys found
                        </div>
                    </div>
                </div>

                <div x-show="currentView === 'tags'" x-cloak>
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-2xl font-bold">Tags</h2>
                        <button @click="refreshTags()" class="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 transition-colors">
                            Refresh
                        </button>
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                        <template x-for="tag in tags" :key="tag.name">
                            <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                                <div class="flex items-center justify-between mb-4">
                                    <h3 class="font-semibold text-white" x-text="tag.name"></h3>
                                    <span class="px-2 py-1 text-xs rounded-full bg-blue-500/20 text-blue-400" x-text="tag.count + ' keys'"></span>
                                </div>
                                <div class="flex gap-2">
                                    <button @click="viewTagKeys(tag.name)" class="flex-1 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors text-sm">
                                        View Keys
                                    </button>
                                    <button @click="invalidateTag(tag.name)" class="flex-1 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 transition-colors text-sm">
                                        Invalidate
                                    </button>
                                </div>
                            </div>
                        </template>
                    </div>
                    <div x-show="tags.length === 0" class="bg-slate-800 rounded-xl p-8 text-center text-slate-400 border border-slate-700">
                        No tags found
                    </div>
                </div>

                <div x-show="currentView === 'namespaces'" x-cloak>
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-2xl font-bold">Namespaces</h2>
                        <button @click="showAddNamespaceModal = true" class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
                            Add Namespace
                        </button>
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                        <template x-for="ns in namespaces" :key="ns.name">
                            <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                                <div class="flex items-center justify-between mb-4">
                                    <h3 class="font-semibold text-white" x-text="ns.name"></h3>
                                    <span x-show="ns.name === 'default'" class="px-2 py-1 text-xs rounded-full bg-green-500/20 text-green-400">Default</span>
                                </div>
                                <div class="space-y-2 text-sm">
                                    <div class="flex justify-between">
                                        <span class="text-slate-400">Keys:</span>
                                        <span class="text-white" x-text="ns.keys"></span>
                                    </div>
                                    <div class="flex justify-between">
                                        <span class="text-slate-400">Memory:</span>
                                        <span class="text-white" x-text="formatBytes(ns.memory)"></span>
                                    </div>
                                </div>
                                <button x-show="ns.name !== 'default'" @click="deleteNamespace(ns.name)" class="w-full mt-4 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 transition-colors text-sm">
                                    Delete
                                </button>
                            </div>
                        </template>
                    </div>
                </div>

                <div x-show="currentView === 'cluster'" x-cloak>
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-2xl font-bold">Cluster</h2>
                        <button @click="showJoinClusterModal = true" class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
                            Join Cluster
                        </button>
                    </div>
                    <div class="bg-slate-800 rounded-xl p-6 border border-slate-700 mb-6">
                        <div class="flex items-center gap-4 mb-6">
                            <div class="w-4 h-4 rounded-full" :class="cluster.state === 'ok' ? 'bg-green-500' : 'bg-red-500'"></div>
                            <span class="text-lg font-semibold" x-text="'Cluster State: ' + cluster.state.toUpperCase()"></span>
                        </div>
                        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div class="bg-slate-700/50 rounded-lg p-4">
                                <p class="text-slate-400 text-sm">Known Nodes</p>
                                <p class="text-2xl font-bold text-white" x-text="cluster.known_nodes"></p>
                            </div>
                            <div class="bg-slate-700/50 rounded-lg p-4">
                                <p class="text-slate-400 text-sm">Cluster Size</p>
                                <p class="text-2xl font-bold text-white" x-text="cluster.size"></p>
                            </div>
                            <div class="bg-slate-700/50 rounded-lg p-4">
                                <p class="text-slate-400 text-sm">Slots Assigned</p>
                                <p class="text-2xl font-bold text-white" x-text="cluster.slots_assigned"></p>
                            </div>
                            <div class="bg-slate-700/50 rounded-lg p-4">
                                <p class="text-slate-400 text-sm">Current Epoch</p>
                                <p class="text-2xl font-bold text-white" x-text="cluster.current_epoch"></p>
                            </div>
                        </div>
                    </div>
                    <h3 class="text-lg font-semibold mb-4">Nodes</h3>
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <template x-for="node in cluster.nodes" :key="node.id">
                            <div class="bg-slate-800 rounded-xl p-6 border border-slate-700">
                                <div class="flex items-center justify-between mb-4">
                                    <span class="font-mono text-sm text-slate-400" x-text="node.id"></span>
                                    <span class="px-2 py-1 text-xs rounded-full" :class="node.role === 'master' ? 'bg-purple-500/20 text-purple-400' : 'bg-blue-500/20 text-blue-400'" x-text="node.role"></span>
                                </div>
                                <div class="space-y-2 text-sm">
                                    <div class="flex justify-between">
                                        <span class="text-slate-400">Address:</span>
                                        <span class="text-white font-mono" x-text="node.addr"></span>
                                    </div>
                                    <div class="flex justify-between">
                                        <span class="text-slate-400">Slots:</span>
                                        <span class="text-white" x-text="node.slots"></span>
                                    </div>
                                    <div class="flex justify-between">
                                        <span class="text-slate-400">Connected:</span>
                                        <span :class="node.connected ? 'text-green-400' : 'text-red-400'" x-text="node.connected ? 'Yes' : 'No'"></span>
                                    </div>
                                </div>
                            </div>
                        </template>
                    </div>
                </div>

                <div x-show="currentView === 'console'" x-cloak>
                    <h2 class="text-2xl font-bold mb-6">Console</h2>
                    <div class="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
                        <div class="bg-slate-700/50 px-4 py-2 flex items-center gap-2">
                            <div class="w-3 h-3 rounded-full bg-red-500"></div>
                            <div class="w-3 h-3 rounded-full bg-yellow-500"></div>
                            <div class="w-3 h-3 rounded-full bg-green-500"></div>
                            <span class="ml-2 text-sm text-slate-400">CacheStorm Console</span>
                        </div>
                        <div id="console-output" class="h-96 overflow-auto p-4 font-mono text-sm bg-slate-900">
                            <template x-for="(line, i) in consoleHistory" :key="i">
                                <div class="mb-1" :class="line.type === 'error' ? 'text-red-400' : (line.type === 'command' ? 'text-green-400' : 'text-slate-300')">
                                    <span x-show="line.type === 'command'" class="text-blue-400">> </span>
                                    <span x-text="line.text"></span>
                                </div>
                            </template>
                        </div>
                        <div class="border-t border-slate-700 p-4 flex gap-2">
                            <input type="text" x-model="consoleInput" @keyup.enter="executeConsoleCommand()"
                                class="flex-1 px-4 py-2 bg-slate-900 border border-slate-700 rounded-lg text-white font-mono placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                placeholder="Enter command (e.g., GET mykey, SET mykey value, KEYS *)">
                            <button @click="executeConsoleCommand()" class="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
                                Execute
                            </button>
                        </div>
                    </div>
                </div>

                <div x-show="currentView === 'slowlog'" x-cloak>
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-2xl font-bold">Slow Log</h2>
                        <button @click="refreshSlowlog()" class="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 transition-colors">
                            Refresh
                        </button>
                    </div>
                    <div class="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
                        <table class="w-full">
                            <thead class="bg-slate-700/50">
                                <tr>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">ID</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Timestamp</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Duration</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-slate-400 uppercase tracking-wider">Command</th>
                                </tr>
                            </thead>
                            <tbody class="divide-y divide-slate-700">
                                <template x-for="entry in slowlog" :key="entry.id">
                                    <tr class="hover:bg-slate-700/30">
                                        <td class="px-6 py-4 text-sm text-slate-400" x-text="entry.id"></td>
                                        <td class="px-6 py-4 text-sm text-slate-300" x-text="entry.start_time"></td>
                                        <td class="px-6 py-4">
                                            <span class="px-2 py-1 text-xs rounded-full bg-yellow-500/20 text-yellow-400" x-text="entry.duration"></span>
                                        </td>
                                        <td class="px-6 py-4 text-sm text-white font-mono" x-text="entry.command"></td>
                                    </tr>
                                </template>
                            </tbody>
                        </table>
                        <div x-show="slowlog.length === 0" class="p-8 text-center text-slate-400">
                            No slow log entries
                        </div>
                    </div>
                </div>
            </main>
        </div>

        <div x-show="showAddKeyModal" x-cloak class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div class="bg-slate-800 rounded-xl p-6 w-full max-w-md border border-slate-700">
                <h3 class="text-lg font-semibold mb-4">Add Key</h3>
                <div class="space-y-4">
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Key</label>
                        <input type="text" x-model="newKey.key" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Value</label>
                        <textarea x-model="newKey.value" rows="3" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
                    </div>
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Type</label>
                        <select x-model="newKey.type" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                            <option value="string">String</option>
                            <option value="list">List</option>
                            <option value="set">Set</option>
                            <option value="hash">Hash</option>
                        </select>
                    </div>
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Tags (comma-separated)</label>
                        <input type="text" x-model="newKey.tags" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                </div>
                <div class="flex gap-3 mt-6">
                    <button @click="showAddKeyModal = false" class="flex-1 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors">Cancel</button>
                    <button @click="addKey()" class="flex-1 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">Add</button>
                </div>
            </div>
        </div>

        <div x-show="showAddNamespaceModal" x-cloak class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div class="bg-slate-800 rounded-xl p-6 w-full max-w-md border border-slate-700">
                <h3 class="text-lg font-semibold mb-4">Add Namespace</h3>
                <div>
                    <label class="block text-sm text-slate-400 mb-1">Name</label>
                    <input type="text" x-model="newNamespace" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                </div>
                <div class="flex gap-3 mt-6">
                    <button @click="showAddNamespaceModal = false" class="flex-1 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors">Cancel</button>
                    <button @click="addNamespace()" class="flex-1 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">Add</button>
                </div>
            </div>
        </div>

        <div x-show="showJoinClusterModal" x-cloak class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div class="bg-slate-800 rounded-xl p-6 w-full max-w-md border border-slate-700">
                <h3 class="text-lg font-semibold mb-4">Join Cluster</h3>
                <div class="space-y-4">
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Host</label>
                        <input type="text" x-model="joinCluster.host" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                    <div>
                        <label class="block text-sm text-slate-400 mb-1">Port</label>
                        <input type="number" x-model="joinCluster.port" class="w-full px-4 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                </div>
                <div class="flex gap-3 mt-6">
                    <button @click="showJoinClusterModal = false" class="flex-1 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors">Cancel</button>
                    <button @click="joinClusterNode()" class="flex-1 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">Join</button>
                </div>
            </div>
        </div>

        <div x-show="showViewKeyModal" x-cloak class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div class="bg-slate-800 rounded-xl p-6 w-full max-w-lg border border-slate-700">
                <h3 class="text-lg font-semibold mb-4">Key Details</h3>
                <div class="space-y-3 font-mono text-sm">
                    <div class="flex justify-between">
                        <span class="text-slate-400">Key:</span>
                        <span class="text-white" x-text="viewingKey.key"></span>
                    </div>
                    <div class="flex justify-between">
                        <span class="text-slate-400">Type:</span>
                        <span class="text-blue-400" x-text="viewingKey.type"></span>
                    </div>
                    <div class="flex justify-between">
                        <span class="text-slate-400">TTL:</span>
                        <span class="text-white" x-text="viewingKey.ttl"></span>
                    </div>
                    <div class="flex justify-between">
                        <span class="text-slate-400">Size:</span>
                        <span class="text-white" x-text="formatBytes(viewingKey.size)"></span>
                    </div>
                    <div>
                        <span class="text-slate-400">Value:</span>
                        <pre class="mt-2 p-3 bg-slate-900 rounded-lg text-slate-300 text-xs overflow-auto max-h-48" x-text="viewingKey.value"></pre>
                    </div>
                </div>
                <button @click="showViewKeyModal = false" class="w-full mt-6 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors">Close</button>
            </div>
        </div>

        <div x-show="showTagKeysModal" x-cloak class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div class="bg-slate-800 rounded-xl p-6 w-full max-w-lg border border-slate-700">
                <h3 class="text-lg font-semibold mb-4">Keys for Tag: <span x-text="viewingTagName"></span></h3>
                <div class="max-h-64 overflow-auto space-y-2">
                    <template x-for="key in viewingTagKeys" :key="key">
                        <div class="px-3 py-2 bg-slate-700/50 rounded-lg font-mono text-sm text-slate-300" x-text="key"></div>
                    </template>
                </div>
                <button @click="showTagKeysModal = false" class="w-full mt-6 py-2 bg-slate-700 text-slate-300 rounded-lg hover:bg-slate-600 transition-colors">Close</button>
            </div>
        </div>

        <div x-show="notification.show" x-cloak
            x-transition:enter="transition ease-out duration-300"
            x-transition:enter-start="opacity-0 transform translate-y-2"
            x-transition:enter-end="opacity-100 transform translate-y-0"
            x-transition:leave="transition ease-in duration-200"
            x-transition:leave-start="opacity-100 transform translate-y-0"
            x-transition:leave-end="opacity-0 transform translate-y-2"
            class="fixed bottom-6 right-6 px-6 py-3 rounded-lg shadow-lg z-50"
            :class="notification.type === 'success' ? 'bg-green-600' : (notification.type === 'error' ? 'bg-red-600' : 'bg-blue-600')">
            <span x-text="notification.message"></span>
        </div>
    </div>

    <script>
        function adminApp() {
            return {
                authenticated: false,
                requiresAuth: true,
                loginPassword: '',
                loginError: '',
                currentView: 'dashboard',
                stats: { keys: 0, memory: 0, tags: 0, namespaces: 0, uptime: '0s', shards: 256, started_at: null },
                keys: [],
                filteredKeys: [],
                keySearch: '',
                tags: [],
                namespaces: [],
                cluster: { state: 'ok', known_nodes: 1, size: 1, slots_assigned: 16384, current_epoch: 1, nodes: [] },
                slowlog: [],
                consoleHistory: [],
                consoleInput: '',
                recentActivity: [],
                topTags: [],
                showAddKeyModal: false,
                showAddNamespaceModal: false,
                showJoinClusterModal: false,
                showViewKeyModal: false,
                showTagKeysModal: false,
                newKey: { key: '', value: '', type: 'string', tags: '' },
                newNamespace: '',
                joinCluster: { host: '127.0.0.1', port: 7946 },
                viewingKey: {},
                viewingTagName: '',
                viewingTagKeys: [],
                notification: { show: false, message: '', type: 'success' },
                refreshInterval: null,

                async init() {
                    try {
                        const resp = await fetch('/api/health');
                        const data = await resp.json();
                        this.requiresAuth = false;
                        this.authenticated = true;
                        await this.loadAll();
                        this.startAutoRefresh();
                    } catch (e) {
                        this.requiresAuth = true;
                    }
                },

                async login() {
                    try {
                        const resp = await fetch('/api/login', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ password: this.loginPassword })
                        });
                        const data = await resp.json();
                        if (data.success) {
                            this.authenticated = true;
                            await this.loadAll();
                            this.startAutoRefresh();
                        } else {
                            this.loginError = 'Invalid password';
                        }
                    } catch (e) {
                        this.loginError = 'Login failed';
                    }
                },

                startAutoRefresh() {
                    this.refreshInterval = setInterval(() => this.refreshStats(), 5000);
                },

                async loadAll() {
                    await Promise.all([
                        this.refreshStats(),
                        this.refreshKeys(),
                        this.refreshTags(),
                        this.refreshNamespaces(),
                        this.refreshCluster()
                    ]);
                },

                getAuthHeaders() {
                    return { 'Content-Type': 'application/json' };
                },

                async refreshStats() {
                    try {
                        const resp = await fetch('/api/stats', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.stats = data;
                    } catch (e) {
                        console.error('Failed to refresh stats:', e);
                    }
                },

                async refreshKeys() {
                    try {
                        const resp = await fetch('/api/keys', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.keys = data.keys || [];
                        this.filteredKeys = this.keys;
                    } catch (e) {
                        console.error('Failed to refresh keys:', e);
                    }
                },

                searchKeys() {
                    const search = this.keySearch.toLowerCase();
                    this.filteredKeys = this.keys.filter(k =>
                        k.key.toLowerCase().includes(search) ||
                        k.type.toLowerCase().includes(search)
                    );
                },

                async refreshTags() {
                    try {
                        const resp = await fetch('/api/tags', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.tags = data.tags || [];
                        this.topTags = this.tags.slice(0, 5);
                    } catch (e) {
                        console.error('Failed to refresh tags:', e);
                    }
                },

                async refreshNamespaces() {
                    try {
                        const resp = await fetch('/api/namespaces', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.namespaces = data.namespaces || [];
                    } catch (e) {
                        console.error('Failed to refresh namespaces:', e);
                    }
                },

                async refreshCluster() {
                    try {
                        const resp = await fetch('/api/cluster', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.cluster = data;
                    } catch (e) {
                        console.error('Failed to refresh cluster:', e);
                    }
                },

                async refreshSlowlog() {
                    try {
                        const resp = await fetch('/api/slowlog', { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.slowlog = data.entries || [];
                    } catch (e) {
                        console.error('Failed to refresh slowlog:', e);
                    }
                },

                async addKey() {
                    try {
                        const tags = this.newKey.tags.split(',').map(t => t.trim()).filter(t => t);
                        const resp = await fetch('/api/keys', {
                            method: 'POST',
                            headers: this.getAuthHeaders(),
                            body: JSON.stringify({ ...this.newKey, tags })
                        });
                        const data = await resp.json();
                        if (data.result === 'OK') {
                            this.showAddKeyModal = false;
                            this.newKey = { key: '', value: '', type: 'string', tags: '' };
                            await this.refreshKeys();
                            await this.refreshStats();
                            this.notify('Key added successfully', 'success');
                        } else {
                            this.notify(data.reason || 'Failed to add key', 'error');
                        }
                    } catch (e) {
                        this.notify('Failed to add key', 'error');
                    }
                },

                async viewKey(key) {
                    try {
                        const resp = await fetch('/api/key/' + encodeURIComponent(key.key), { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.viewingKey = data;
                        this.showViewKeyModal = true;
                    } catch (e) {
                        this.notify('Failed to load key details', 'error');
                    }
                },

                async deleteKey(key) {
                    if (!confirm('Delete key "' + key + '"?')) return;
                    try {
                        const resp = await fetch('/api/key/' + encodeURIComponent(key), {
                            method: 'DELETE',
                            headers: this.getAuthHeaders()
                        });
                        const data = await resp.json();
                        if (data.deleted) {
                            await this.refreshKeys();
                            await this.refreshStats();
                            this.notify('Key deleted', 'success');
                        }
                    } catch (e) {
                        this.notify('Failed to delete key', 'error');
                    }
                },

                async viewTagKeys(tag) {
                    try {
                        const resp = await fetch('/api/tag/' + encodeURIComponent(tag), { headers: this.getAuthHeaders() });
                        const data = await resp.json();
                        this.viewingTagName = tag;
                        this.viewingTagKeys = data.keys || [];
                        this.showTagKeysModal = true;
                    } catch (e) {
                        this.notify('Failed to load tag keys', 'error');
                    }
                },

                async invalidateTag(tag) {
                    if (!confirm('Invalidate all keys with tag "' + tag + '"?')) return;
                    try {
                        const resp = await fetch('/api/invalidate/' + encodeURIComponent(tag), {
                            method: 'POST',
                            headers: this.getAuthHeaders()
                        });
                        const data = await resp.json();
                        await this.refreshKeys();
                        await this.refreshTags();
                        await this.refreshStats();
                        this.notify('Invalidated ' + data.keys_deleted + ' keys', 'success');
                    } catch (e) {
                        this.notify('Failed to invalidate tag', 'error');
                    }
                },

                async addNamespace() {
                    if (!this.newNamespace) return;
                    try {
                        const resp = await fetch('/api/namespaces', {
                            method: 'POST',
                            headers: this.getAuthHeaders(),
                            body: JSON.stringify({ name: this.newNamespace })
                        });
                        const data = await resp.json();
                        if (data.result === 'OK') {
                            this.showAddNamespaceModal = false;
                            this.newNamespace = '';
                            await this.refreshNamespaces();
                            this.notify('Namespace created', 'success');
                        }
                    } catch (e) {
                        this.notify('Failed to create namespace', 'error');
                    }
                },

                async deleteNamespace(name) {
                    if (!confirm('Delete namespace "' + name + '"?')) return;
                    try {
                        const resp = await fetch('/api/namespace/' + encodeURIComponent(name), {
                            method: 'DELETE',
                            headers: this.getAuthHeaders()
                        });
                        const data = await resp.json();
                        if (data.result === 'OK') {
                            await this.refreshNamespaces();
                            this.notify('Namespace deleted', 'success');
                        } else {
                            this.notify(data.reason || 'Failed to delete namespace', 'error');
                        }
                    } catch (e) {
                        this.notify('Failed to delete namespace', 'error');
                    }
                },

                async joinClusterNode() {
                    try {
                        const resp = await fetch('/api/cluster/join', {
                            method: 'POST',
                            headers: this.getAuthHeaders(),
                            body: JSON.stringify(this.joinCluster)
                        });
                        const data = await resp.json();
                        this.showJoinClusterModal = false;
                        this.notify(data.message || 'Joined cluster', 'success');
                        await this.refreshCluster();
                    } catch (e) {
                        this.notify('Failed to join cluster', 'error');
                    }
                },

                async executeConsoleCommand() {
                    if (!this.consoleInput.trim()) return;
                    const input = this.consoleInput.trim();
                    this.consoleHistory.push({ type: 'command', text: input });
                    this.consoleInput = '';

                    const parts = input.split(/\s+/);
                    const cmd = parts[0].toUpperCase();
                    const args = parts.slice(1);

                    try {
                        const resp = await fetch('/api/execute', {
                            method: 'POST',
                            headers: this.getAuthHeaders(),
                            body: JSON.stringify({ command: cmd, args: args })
                        });
                        const data = await resp.json();
                        let result = data.result;
                        if (typeof result === 'object') {
                            result = JSON.stringify(result, null, 2);
                        }
                        this.consoleHistory.push({ type: 'result', text: result !== null ? String(result) : '(nil)' });
                    } catch (e) {
                        this.consoleHistory.push({ type: 'error', text: 'Error: ' + e.message });
                    }

                    this.$nextTick(() => {
                        const el = document.getElementById('console-output');
                        if (el) el.scrollTop = el.scrollHeight;
                    });
                },

                formatBytes(bytes) {
                    if (bytes === 0) return '0 B';
                    const k = 1024;
                    const sizes = ['B', 'KB', 'MB', 'GB'];
                    const i = Math.floor(Math.log(bytes) / Math.log(k));
                    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
                },

                notify(message, type = 'success') {
                    this.notification = { show: true, message, type };
                    setTimeout(() => { this.notification.show = false; }, 3000);
                }
            };
        }
    </script>
</body>
</html>
`
