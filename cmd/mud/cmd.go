package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var commands = map[string]func(*Session, ...string){
	"/dump":             dump,
	"/set":              set,
	"/vars":             vars,
	"/aliases":          aliases,
	"/tick":             tick,
	"/wait":             wait,
	"/disable-triggers": disableTriggers,
	"/enable-triggers":  enableTriggers,
	"/history":          history,
	"/clear-history":    clearHistory,
}

func dump(c *Session, args ...string) {
	if len(args) != 1 {
		fmt.Fprintf(c.output, "dump: missing argument\n")
		return
	}
	cfg := c.cfg.Dump[args[0]]
	fmt.Fprintln(c.conn, cfg.Cmd)
	fmt.Fprintf(c.output, "/dump %s\n", cfg.Cmd)

	dumpc := make(chan []byte)

	c.Lock()
	c.dumpc = dumpc
	c.Unlock()

	f, err := os.Create(filepath.Join(c.path, cfg.Dest))
	if err != nil {
		fmt.Fprintf(c.output, "dump: %v\n", err)
		return
	}

	go func() {
		dumping := false
		for line := range dumpc {
			if cfg.Start.Match(line) {
				dumping = true
			}
			if dumping {
				fmt.Fprintln(f, string(line))
			}
			if cfg.End.Match(line) {
				c.Lock()
				c.dumpc = nil
				c.Unlock()
				return
			}
		}
	}()
}

func set(c *Session, args ...string) {
	for _, arg := range args {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			continue
		}
		c.Lock()
		c.vars[parts[0]] = parts[1]
		c.Unlock()
		fmt.Fprintf(c.output, "%s=%s\n", parts[0], parts[1])
	}
}

func vars(c *Session, args ...string) {
	c.RLock()
	for name, val := range c.vars {
		fmt.Fprintf(c.output, "%s=%s\n", name, val)
	}
	c.RUnlock()
}

func aliases(c *Session, args ...string) {
	c.RLock()
	for name, val := range c.cfg.Aliases {
		fmt.Fprintf(c.output, "%s=%s\n", name, val)
	}
	c.RUnlock()
}

func tick(c *Session, args ...string) {
	c.RLock()
	defer c.RUnlock()

	if c.lastTick.IsZero() {
		info.Fprintln(c.output, "no tick info")
		return
	}
	var next time.Time
	for next = c.lastTick; next.Before(time.Now()); next = next.Add(c.cfg.Tick.Duration) {
	}
	info.Fprintf(c.output, "next tick in %ds\n", int(next.Sub(time.Now()).Seconds()))
}

func wait(c *Session, args ...string) {
	if len(args) == 0 {
		return
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return
	}

	// no way to interrupt, so max out at 5 seconds
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	time.Sleep(d)
}

func disableTriggers(c *Session, args ...string) {
	c.Lock()
	c.triggersDisabled = true
	c.Unlock()
}

func enableTriggers(c *Session, args ...string) {
	c.Lock()
	c.triggersDisabled = false
	c.Unlock()
}

func history(c *Session, args ...string) {
	fmt.Println(strings.Join(c.history, "; "))
}

func clearHistory(c *Session, args ...string) {
	c.history = []string{}
}
