package store

import (
	"sync"
	"time"
)

type Event struct {
	ID        string
	Name      string
	Data      map[string]interface{}
	Timestamp int64
}

type Webhook struct {
	ID      string
	URL     string
	Method  string
	Headers map[string]string
	Events  []string
	Enabled bool
	LastHit int64
	Hits    int64
	Errors  int64
}

type EventManager struct {
	Events    []*Event
	Webhooks  map[string]*Webhook
	Listeners map[string][]chan Event
	mu        sync.RWMutex
}

func NewEventManager() *EventManager {
	return &EventManager{
		Events:    make([]*Event, 0),
		Webhooks:  make(map[string]*Webhook),
		Listeners: make(map[string][]chan Event),
	}
}

func (em *EventManager) Emit(name string, data map[string]interface{}) *Event {
	em.mu.Lock()
	defer em.mu.Unlock()

	event := &Event{
		ID:        generateID(),
		Name:      name,
		Data:      data,
		Timestamp: currentTimeMillis(),
	}

	em.Events = append(em.Events, event)
	if len(em.Events) > 1000 {
		em.Events = em.Events[len(em.Events)-1000:]
	}

	if listeners, ok := em.Listeners[name]; ok {
		for _, ch := range listeners {
			select {
			case ch <- *event:
			default:
			}
		}
	}

	return event
}

func (em *EventManager) Subscribe(name string) chan Event {
	em.mu.Lock()
	defer em.mu.Unlock()

	ch := make(chan Event, 100)
	em.Listeners[name] = append(em.Listeners[name], ch)
	return ch
}

func (em *EventManager) Unsubscribe(name string, ch chan Event) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if listeners, ok := em.Listeners[name]; ok {
		for i, listener := range listeners {
			if listener == ch {
				em.Listeners[name] = append(listeners[:i], listeners[i+1:]...)
				close(ch)
				break
			}
		}
	}
}

func (em *EventManager) GetEvents(name string, limit int) []*Event {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var result []*Event
	for i := len(em.Events) - 1; i >= 0 && len(result) < limit; i-- {
		if name == "" || em.Events[i].Name == name {
			result = append(result, em.Events[i])
		}
	}
	return result
}

func (em *EventManager) CreateWebhook(id, url, method string, events []string) *Webhook {
	em.mu.Lock()
	defer em.mu.Unlock()

	wh := &Webhook{
		ID:      id,
		URL:     url,
		Method:  method,
		Events:  events,
		Headers: make(map[string]string),
		Enabled: true,
	}

	em.Webhooks[id] = wh
	return wh
}

func (em *EventManager) GetWebhook(id string) (*Webhook, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	wh, ok := em.Webhooks[id]
	return wh, ok
}

func (em *EventManager) DeleteWebhook(id string) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	if _, exists := em.Webhooks[id]; !exists {
		return false
	}
	delete(em.Webhooks, id)
	return true
}

func (em *EventManager) ListWebhooks() []*Webhook {
	em.mu.RLock()
	defer em.mu.RUnlock()

	result := make([]*Webhook, 0, len(em.Webhooks))
	for _, wh := range em.Webhooks {
		result = append(result, wh)
	}
	return result
}

func (em *EventManager) EnableWebhook(id string) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	if wh, ok := em.Webhooks[id]; ok {
		wh.Enabled = true
		return true
	}
	return false
}

func (em *EventManager) DisableWebhook(id string) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	if wh, ok := em.Webhooks[id]; ok {
		wh.Enabled = false
		return true
	}
	return false
}

func (em *EventManager) RecordWebhookHit(id string, success bool) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if wh, ok := em.Webhooks[id]; ok {
		wh.LastHit = currentTimeMillis()
		wh.Hits++
		if !success {
			wh.Errors++
		}
	}
}

func (wh *Webhook) SetHeader(key, value string) {
	wh.Headers[key] = value
}

func (wh *Webhook) Stats() map[string]interface{} {
	return map[string]interface{}{
		"id":       wh.ID,
		"url":      wh.URL,
		"method":   wh.Method,
		"events":   wh.Events,
		"enabled":  wh.Enabled,
		"last_hit": wh.LastHit,
		"hits":     wh.Hits,
		"errors":   wh.Errors,
	}
}

func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	id := make([]byte, 16)
	for i := range id {
		id[i] = chars[absInt(fastRand(int64(len(chars))))]
	}
	return string(id)
}

func fastRand(n int64) int64 {
	seed := currentTimeMillis()
	return (seed*1103515245 + 12345) % n
}

func absInt(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func currentTimeMillis() int64 {
	return nanoTime() / 1e6
}

func nanoTime() int64 {
	return time.Now().UnixNano()
}

var _ = nanoTime

type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Name() string
}

type RLECompressor struct{}

func (c *RLECompressor) Name() string { return "rle" }

func (c *RLECompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	result := make([]byte, 0)
	i := 0

	for i < len(data) {
		b := data[i]
		count := 1

		for i+count < len(data) && data[i+count] == b && count < 255 {
			count++
		}

		result = append(result, byte(count), b)
		i += count
	}

	return result, nil
}

func (c *RLECompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	result := make([]byte, 0)

	for i := 0; i < len(data); i += 2 {
		if i+1 >= len(data) {
			break
		}
		count := int(data[i])
		b := data[i+1]

		for j := 0; j < count; j++ {
			result = append(result, b)
		}
	}

	return result, nil
}

type LZ4Compressor struct{}

func (c *LZ4Compressor) Name() string { return "lz4" }

func (c *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	return lz4Compress(data), nil
}

func (c *LZ4Compressor) Decompress(data []byte) ([]byte, error) {
	return lz4Decompress(data), nil
}

func lz4Compress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	result := make([]byte, 0)
	window := make(map[string]int)
	pos := 0

	for pos < len(data) {
		matchLen := 0
		matchPos := 0

		for l := min(12, len(data)-pos); l >= 4; l-- {
			key := string(data[pos : pos+l])
			if idx, ok := window[key]; ok && pos-idx < 65536 {
				matchLen = l
				matchPos = idx
				break
			}
		}

		if matchLen >= 4 {
			offset := pos - matchPos
			token := byte((min(matchLen-4, 15) << 4) | 0)
			result = append(result, token, byte(offset&0xFF), byte(offset>>8))

			if matchLen-4 >= 15 {
				extra := matchLen - 4 - 15
				for extra >= 255 {
					result = append(result, 255)
					extra -= 255
				}
				result = append(result, byte(extra))
			}

			for i := 0; i < matchLen && pos < len(data); i++ {
				key := string(data[pos : pos+min(4, len(data)-pos)])
				window[key] = pos
				pos++
			}
		} else {
			literal := data[pos]
			result = append(result, 0, literal)
			key := string(data[pos : pos+min(4, len(data)-pos)])
			window[key] = pos
			pos++
		}
	}

	return result
}

func lz4Decompress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	result := make([]byte, 0)
	pos := 0

	for pos < len(data) {
		token := data[pos]
		pos++

		literalLen := int(token >> 4)
		matchLen := int(token&0x0F) + 4

		if literalLen == 15 {
			for pos < len(data) {
				extra := int(data[pos])
				pos++
				literalLen += extra
				if extra < 255 {
					break
				}
			}
		}

		for i := 0; i < literalLen && pos < len(data); i++ {
			result = append(result, data[pos])
			pos++
		}

		if pos >= len(data) {
			break
		}

		if pos+1 >= len(data) {
			break
		}
		offset := int(data[pos]) | int(data[pos+1])<<8
		pos += 2

		if matchLen == 19 {
			for pos < len(data) {
				extra := int(data[pos])
				pos++
				matchLen += extra
				if extra < 255 {
					break
				}
			}
		}

		start := len(result) - offset
		for i := 0; i < matchLen && start+i >= 0; i++ {
			if start+i < len(result) {
				result = append(result, result[start+i])
			}
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var GlobalEventManager = NewEventManager()
