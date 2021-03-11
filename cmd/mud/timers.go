package main

import (
	"context"
	"net"
	"time"
)

type Timer struct {
	Every time.Duration
	Do    string
}

func (t *Timer) start(ctx context.Context, conn net.Conn) {
	ticker := time.NewTicker(t.Every)
	go func() {
		for {
			select {
			case <-ticker.C:
				conn.Write([]byte(t.Do + "\n"))
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *Session) startTimers() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	for _, t := range c.cfg.Timers {
		t.start(ctx, c.conn)
	}
	return cancel
}
