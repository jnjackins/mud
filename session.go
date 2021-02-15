package mud

// TODO: var substitution
// TODO: regexp substitutions

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/buildkite/interpolate"
	"github.com/fatih/color"
)

type Session struct {
	conn  net.Conn
	input io.Reader

	output io.Writer
	log    io.Writer

	sync.RWMutex
	cfg              Config
	triggersDisabled bool

	cancelTimers context.CancelFunc
	lastTick     time.Time
	vars         *mapvars
}

type mapvars struct {
	sync.Mutex
	m map[string]string
}

func (m *mapvars) Get(key string) (string, bool) {
	v, ok := m.m[key]
	return v, ok
}

var (
	highlight = color.New(color.FgHiMagenta)
	info      = color.New(color.FgBlue)
)

func New(conn net.Conn, input io.Reader, output, log io.Writer) *Session {
	s := &Session{
		conn:   conn,
		input:  input,
		output: output,
		log:    log,
		vars: &mapvars{
			m: make(map[string]string),
		},
	}

	errors := make(chan error)
	tickCh := make(chan time.Time)

	go func() {
		errors <- s.receive(tickCh)
	}()

	go func() {
		errors <- s.send()
	}()

	go s.notifyTicks(tickCh)

	go s.Login()

	return s
}

func (c *Session) receive(tick chan<- time.Time) error {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Bytes()

		if c.isLog(line) {
			ts := time.Now().Format(time.Kitchen)
			fmt.Fprintf(c.log, "%s %s\n", ts, string(line))
		}
		if c.isHighlight(line) {
			highlight.Fprintln(c.output, string(line))
		} else {
			fmt.Fprintln(c.output, string(line))
		}

		c.triggers(line)

		// tick timer
		c.RLock()
		for _, s := range c.cfg.Tick.Match {
			if bytes.Contains(line, []byte(s)) {
				tick <- time.Now()
				break
			}
		}
		c.RUnlock()
	}
	return scanner.Err()
}

func (c *Session) triggers(line []byte) {
	c.RLock()
	defer c.RUnlock()

	if c.triggersDisabled {
		return
	}
	var wait time.Duration
	for pattern := range c.cfg.Triggers {
		if pattern.match(line) {
			// expand regexp capture groups
			template := c.cfg.Triggers[pattern]
			s := pattern.expand(string(line), template)

			info.Fprintf(c.output, "[trigger: %s]\n", s)

			// expand aliases
			c.vars.Lock()
			for _, sub := range c.expand(s) {
				sub, _ := interpolate.Interpolate(c.vars, sub)
				go func(s string, wait time.Duration) {
					time.Sleep(wait)
					fmt.Fprintf(c.conn, "%s\n", s)
				}(sub, wait)
				wait += 100 * time.Millisecond
			}
			c.vars.Unlock()

		}
	}
}

func (c *Session) isLog(line []byte) bool {
	for _, re := range c.cfg.Log {
		if re.match(line) {
			return true
		}
	}
	return false
}

func (c *Session) isHighlight(line []byte) bool {
	for _, re := range c.cfg.Highlight {
		if re.match(line) {
			return true
		}
	}
	return false
}

func (c *Session) isPrompt(line []byte) bool {
	if c.cfg.Prompt == "" {
		return false
	}
	return c.cfg.Prompt.match(line)
}

func (c *Session) send() error {
	go func() {
		// TODO: only if nothing else has been input for 5m
		for range time.Tick(5 * time.Minute) {
			c.conn.Write([]byte("\n"))
		}
	}()

	out := io.MultiWriter(c.conn, c.output)
	scanner := bufio.NewScanner(c.input)
	for scanner.Scan() {
		s := scanner.Text()

		for _, sub := range c.expand(s) {
			c.vars.Lock()
			sub, _ := interpolate.Interpolate(c.vars, sub)
			c.vars.Unlock()

			// check for command
			if len(sub) > 0 && sub[0] == '/' {
				fields := strings.Fields(sub)
				if f, ok := commands[fields[0]]; ok {
					fmt.Fprintln(c.output, s)
					f(c, fields[1:]...)
					c.conn.Write([]byte("\n")) // get a fresh prompt
					continue
				}
			}

			fmt.Fprintln(out, sub)
		}
	}
	return scanner.Err()
}

func (c *Session) expand(s string) []string {
	var out []string
	for _, sub := range strings.Split(s, ";") {
		sub := strings.TrimSpace(sub)

		// replace aliases
		words := strings.Fields(sub)
		if len(words) > 0 {
			c.RLock()
			if v, ok := c.cfg.Aliases[words[0]]; ok {
				info.Fprintf(c.output, "[alias: %s]\n", v)
				sub = v + " " + strings.Join(words[1:], " ")
			}
			c.RUnlock()
		}

		if strings.Contains(sub, ";") {
			out = append(out, c.expand(sub)...)
		} else {
			out = append(out, sub)
		}
	}
	return out
}

func (c *Session) notifyTicks(tick <-chan time.Time) {
	for t := range tick {
		time.AfterFunc(c.cfg.Tick.Duration-10*time.Second, func() {
			highlight.Fprintln(c.output, "10s until next tick.")
		})
		c.Lock()
		c.lastTick = t
		c.Unlock()
	}
}

func (c *Session) Login() {
	c.RLock()
	defer c.RUnlock()

	if c.cfg.Login.Name != "" {
		fmt.Fprintln(c.conn, c.cfg.Login.Name)
	}
	if c.cfg.Login.Password != "" {
		fmt.Fprintln(c.conn, c.cfg.Login.Password)
	}
}
