package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

type HTTPServer struct {
	store   *store.Store
	server  *http.Server
	started time.Time
}

func NewHTTPServer(s *store.Store, port int) *HTTPServer {
	h := &HTTPServer{
		store:   s,
		started: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/info", h.handleInfo)
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/keys", h.handleKeys)
	mux.HandleFunc("/tags", h.handleTags)
	mux.HandleFunc("/memory", h.handleMemory)

	h.server = &http.Server{
		Addr:    ":" + string(rune(port)),
		Handler: mux,
	}

	return h
}

func (h *HTTPServer) Start() error {
	return h.server.ListenAndServe()
}

func (h *HTTPServer) Stop() error {
	return h.server.Close()
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(h.started).String(),
	})
}

func (h *HTTPServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"server": map[string]interface{}{
			"version":    "1.0.0",
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

	json.NewEncoder(w).Encode(info)
}

func (h *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	metrics := `# HELP cachestorm_keys_total Total number of keys
# TYPE cachestorm_keys_total gauge
cachestorm_keys_total %d
# HELP cachestorm_memory_bytes Memory usage in bytes
# TYPE cachestorm_memory_bytes gauge
cachestorm_memory_bytes %d
`
	w.Write([]byte(metrics))
}

func (h *HTTPServer) handleKeys(w http.ResponseWriter, r *http.Request) {
	keys := h.store.Keys()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(keys),
		"keys":  keys,
	})
}

func (h *HTTPServer) handleTags(w http.ResponseWriter, r *http.Request) {
	tagIndex := h.store.GetTagIndex()
	if tagIndex == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": []string{},
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(tags),
		"tags":  tagInfo,
	})
}

func (h *HTTPServer) handleMemory(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"used":     h.store.MemUsage(),
		"keys":     h.store.KeyCount(),
		"avg_size": h.avgEntrySize(),
	})
}

func (h *HTTPServer) avgEntrySize() int64 {
	count := h.store.KeyCount()
	if count == 0 {
		return 0
	}
	return h.store.MemUsage() / count
}
