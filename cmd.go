package main

import (
	"fmt"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/jnjackins/mud/telnet"
)

var commands = map[string]func(){
	"/t": nextTick,
}

var lastTick time.Time

func tick(conn *telnet.Conn, config *config, tick <-chan time.Time) {
	for t := range tick {
		if *debug && !lastTick.IsZero() {
			fmt.Printf("%v since last tick\n", t.Sub(lastTick))
		}
		time.AfterFunc(config.Tick.Duration-10*time.Second, func() {
			color.HiMagenta("10s until next tick.")

			// force the prompt to refresh
			syscall.Kill(0, syscall.SIGWINCH)
		})
		lastTick = t
	}
}

func nextTick() {
	if lastTick.IsZero() {
		color.HiMagenta("no tick info")
		return
	}
	var next time.Time
	for next = lastTick; next.Before(time.Now()); next = next.Add(time.Minute) {
	}
	color.HiMagenta("next tick in %ds", int(next.Sub(time.Now()).Seconds()))
}
