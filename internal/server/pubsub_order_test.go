package server

import (
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// TestPubSubMessagesKeepOrderAroundReplies publishes numbered messages while
// the subscriber keeps sending PING: the delivery goroutine and the command
// loop both write queued messages, and neither may write a later message
// before an earlier one.
func TestPubSubMessagesKeepOrderAroundReplies(t *testing.T) {
	srv := newPSServer(t)
	sub := srv.dial()
	pub := srv.dial()
	sub.do("*3\r\n$9\r\nsubscribe\r\n$2\r\nch\r\n:1\r\n", "SUBSCRIBE", "ch")

	const n = 2000
	go func() {
		var batch []byte
		for i := 0; i < n; i++ {
			batch = append(batch, encodeCmd("PUBLISH", "ch", strconv.Itoa(i))...)
			if i%50 == 49 {
				if _, err := pub.conn.Write(batch); err != nil {
					return
				}
				batch = batch[:0]
			}
		}
	}()
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		for {
			select {
			case <-stopPing:
				return
			default:
			}
			if _, err := sub.conn.Write(encodeCmd("PING")); err != nil {
				return
			}
			time.Sleep(50 * time.Microsecond)
		}
	}()

	_ = sub.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	rd := resp.NewReader(sub.r)
	next := 0
	for next < n {
		v, err := rd.ReadValue()
		if err != nil {
			t.Fatalf("after message %d: %v", next-1, err)
		}
		if len(v.Array) != 3 || string(v.Array[0].Bulk) != "message" {
			continue
		}
		if got := string(v.Array[2].Bulk); got != strconv.Itoa(next) {
			t.Fatalf("message %s arrived when %d was expected", got, next)
		}
		next++
	}
}
