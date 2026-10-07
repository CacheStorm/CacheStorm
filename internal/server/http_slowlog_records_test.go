package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/store"
)

type slowlogObservationX struct {
	Name      string
	Want, Got any
}

func slowlogObservationsX(t *testing.T, verify bool) []slowlogObservationX {
	t.Helper()
	old := store.GlobalSlowLog
	store.GlobalSlowLog = store.NewSlowLog(128)
	t.Cleanup(func() { store.GlobalSlowLog = old })
	h := NewHTTPServer(store.NewStore(), command.NewRouter(), &HTTPConfig{})
	t.Cleanup(h.cancel)
	fetch := func(query string) map[string]any {
		w := httptest.NewRecorder()
		h.handleSlowlog(w, httptest.NewRequest("GET", "/api/slowlog"+query, nil))
		if w.Code != 200 {
			t.Fatalf("HTTP status %d", w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	empty := fetch("")
	results := []slowlogObservationX{{"control: empty log", float64(0), empty["count"]}}
	store.GlobalSlowLog.Add(25*time.Millisecond, "GET", [][]byte{[]byte("alpha")}, "127.0.0.1")
	store.GlobalSlowLog.Add(15*time.Millisecond, "GET", [][]byte{[]byte("beta")}, "127.0.0.1")
	results = append(results, slowlogObservationX{"control: native log stores two", 2, store.GlobalSlowLog.Len()})
	body := fetch("?count=1")
	results = append(results, slowlogObservationX{"HTTP exposes recorded count", float64(1), body["count"]})
	if verify {
		entries, ok := body["entries"].([]any)
		if !ok || len(entries) != 1 {
			t.Fatalf("expected one entry: %v", body)
		}
		row := entries[0].(map[string]any)
		results = append(results, slowlogObservationX{"newest entry", "GET beta", row["command"]}, slowlogObservationX{"duration formatting", "15ms", row["duration"]}, slowlogObservationX{"timestamp present", true, row["start_time"] != nil})
		all := fetch("")
		results = append(results, slowlogObservationX{"default count", float64(2), all["count"]})
		large := fetch("?count=999")
		results = append(results, slowlogObservationX{"oversize count clamped", float64(2), large["count"]})
		store.GlobalSlowLog.Clear()
		cleared := fetch("?count=1")
		results = append(results, slowlogObservationX{"cleared log", float64(0), cleared["count"]})
	}
	return results
}
func TestHTTPRecordedSlowlogX(t *testing.T) {
	for _, row := range slowlogObservationsX(t, true) {
		if !reflect.DeepEqual(row.Want, row.Got) {
			t.Errorf("%s: got %#v, want %#v", row.Name, row.Got, row.Want)
		}
	}
}
