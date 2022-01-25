package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jnjackins/mud"
	"github.com/jnjackins/mud/internal/interpolate"
)

var commands = map[string]func(*Session, ...string){
	"/on":            on,
	"/set":           set,
	"/incr":          incr,
	"/vars":          vars,
	"/list":          list,
	"/aliases":       aliases,
	"/wait":          wait,
	"/triggers-off":  disableTriggers,
	"/triggers-on":   enableTriggers,
	"/history":       history,
	"/clear-history": clearHistory,
}

func on(c *Session, args ...string) {
	if len(args) != 2 {
		fmt.Fprintf(c.output, "on: usage: /on {pattern} {action}\n")
		return
	}
	on := mud.Pattern(args[0])
	c.Lock()
	c.oneTimeTriggers[on] = args[1]
	c.Unlock()
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
		// fmt.Fprintf(c.output, "%s=%s\n", parts[0], parts[1])
	}
}

func incr(c *Session, args ...string) {
	if len(args) != 2 {
		fmt.Fprintf(c.output, "incr: usage: /incr var n\n")
		return
	}

	var n1, n2 int64
	var err error

	c.RLock()
	v := c.vars[args[0]]
	c.RUnlock()
	if v == "" {
		n1 = 0
	} else {
		n1, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			fmt.Fprintf(c.output, "failed to parse integer: %v\n", v)
			return
		}
	}

	n2, err = strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		fmt.Fprintf(c.output, "failed to parse integer: %v\n", args[1])
		return
	}

	c.Lock()
	c.vars[args[0]] = strconv.FormatInt(n1+n2, 10)
	c.Unlock()
}

func vars(c *Session, args ...string) {
	c.RLock()
	for name, val := range c.vars {
		fmt.Fprintf(c.output, "%s=%s\n", name, val)
	}
	c.RUnlock()
}

func list(c *Session, args ...string) {
	if len(args) < 2 {
		fmt.Fprintf(c.output, "incr: usage: /list <name> <cmd>\n")
		return
	}

	name := args[0]

	switch args[1] {
	case "print":
		c.RLock()
		list := c.lists[name]
		c.RUnlock()

		fmt.Fprintln(c.output, list)
	case "get":
		if len(args) != 4 {
			fmt.Fprintf(c.output, "incr: usage: /list <name> get <index> <dest>\n")
			return
		}

		c.RLock()
		list := c.lists[name]
		c.RUnlock()

		interpolated, err := interpolate.Interpolate(c.vars, nil, args[2])
		if err != nil {
			fmt.Fprintf(c.output, "failed to interpolate %q\n", args[2])
		}
		i, err := strconv.Atoi(interpolated)
		if err != nil {
			fmt.Fprintf(c.output, "failed to parse integer: %v\n", interpolated)
			return
		}

		if i >= len(list) {
			fmt.Fprintf(c.output, "index %d is out of bounds\n", i)
			return
		}

		c.Lock()
		c.vars[args[3]] = list[i]
		c.Unlock()
		fmt.Fprintf(c.output, "%s=%s\n", args[3], list[i])

	case "push":
		if len(args) != 3 {
			fmt.Fprintf(c.output, "incr: usage: /list <name> push <value>\n")
			return
		}

		val := args[2]

		c.Lock()
		c.lists[name] = append(c.lists[name], val)
		c.Unlock()

		fmt.Fprintf(c.output, "push %s\n", val)

	case "pop":
		if len(args) != 3 {
			fmt.Fprintf(c.output, "incr: usage: /list <name> pop <dest>\n")
			return
		}

		c.RLock()
		list := c.lists[name]
		var val string
		if len(list) > 0 {
			val = list[len(list)-1]
		}
		c.RUnlock()

		key := args[2]

		c.Lock()
		c.vars[key] = val
		c.lists[name] = list[:len(list)-1]
		c.Unlock()

		fmt.Fprintf(c.output, "%s=%s\n", key, val)
	}
}

func aliases(c *Session, args ...string) {
	c.RLock()
	for name, val := range c.cfg.Aliases {
		fmt.Fprintf(c.output, "%s=%s\n", name, val)
	}
	c.RUnlock()
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
	fmt.Fprintf(c.output, "%s\n", strings.Join(c.history, "; "))
}

func clearHistory(c *Session, args ...string) {
	c.history = []string{}
}
