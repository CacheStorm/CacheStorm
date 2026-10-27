package cachestorm

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

type delayedDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c delayedDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func TestSocketDeadlineBeforeContextTimer(t *testing.T) {
	p := &pool{}
	timeout := &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}
	ctx := delayedDeadlineContext{Context: context.Background(), deadline: time.Now().Add(-time.Second)}
	if err := p.mapErr(ctx, timeout); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired context deadline surfaced as socket error: %v", err)
	}
	ctx.deadline = time.Now().Add(time.Hour)
	if err := p.mapErr(ctx, timeout); err != timeout {
		t.Fatalf("early socket timeout changed: %v", err)
	}
}
