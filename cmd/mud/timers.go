package main

import (
	"context"
	"time"
)

func (c *Session) startTimers() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	for _, t := range c.cfg.Timers {
		c.startTimer(ctx, t.Every, t.Do)
	}
	return cancel
}

func (c *Session) startTimer(ctx context.Context, d time.Duration, cmd string) {
	ticker := time.NewTicker(d)
	cmds := c.expand(cmd)
	go func() {
		for {
			select {
			case <-ticker.C:
				for _, s := range cmds {
					c.conn.Write([]byte(s + "\n"))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
