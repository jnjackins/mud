package main

// TODO: multi-input without tell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fvbock/trie"
	"github.com/jnjackins/mud"
	"github.com/jnjackins/mud/internal/interpolate"

	"github.com/fatih/color"
)

var (
	alert = color.New(color.FgHiMagenta)
	info  = color.New(color.FgBlue)
)

type pipe struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (p pipe) Close() error {
	p.r.Close()
	p.w.Close()
	return nil
}

func (p pipe) Read(b []byte) (int, error) {
	return p.r.Read(b)
}

func (p pipe) Write(b []byte) (int, error) {
	return p.w.Write(b)
}

type mapvars map[string]string

func (m mapvars) Get(key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		v = "$" + key
	}
	return v, true
}

type Session struct {
	prefix string
	path   string

	conn   net.Conn
	input  pipe
	output pipe

	sync.RWMutex
	cfg              mud.Config
	vars             mapvars
	history          []string
	lastTick         time.Time
	triggersDisabled bool
	cancelTimers     context.CancelFunc
	dumpc            chan []byte

	// tab completion
	words       *trie.Trie
	expireQueue chan string
}

func (s *Session) Close() error {
	s.input.Close()
	s.output.Close()
	return nil
}

func (s *Session) Start() error {
	tickCh := make(chan time.Time)
	errors := make(chan error)

	go func() {
		errors <- s.receive(tickCh)
	}()

	go func() {
		errors <- s.send()
	}()

	go s.notifyTicks(tickCh)

	go func() {
		if err := s.Login(); err != nil {
			errors <- err
		}
	}()

	return <-errors
}

func (c *Session) receive(tick chan<- time.Time) error {
	scanner := bufio.NewScanner(c.conn)

	logch, err := c.startLogWriter()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		logch <- line

		c.RLock()
		if c.dumpc != nil {
			c.dumpc <- line
		}

		if s, ok := c.replace(line); ok {
			line = s
		} else if s, ok := c.highlight(line); ok {
			line = s
		}
		if !c.gag(line) {
			fmt.Fprintln(c.output, string(line))
		}

		c.triggers(line)

		// tick timer
		for _, s := range c.cfg.Tick.Match {
			if bytes.Contains(line, []byte(s)) {
				// can't synchronously write to channel while holding lock
				go func() {
					tick <- time.Now()
				}()
				break
			}
		}
		c.RUnlock()
	}
	return scanner.Err()
}

func (c *Session) startLogWriter() (chan []byte, error) {
	files := make(map[string]*os.File)
	c.RLock()
	for filename := range c.cfg.Log {
		path := filepath.Join(c.path, filename)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
		files[filename] = f
	}
	c.RUnlock()

	ch := make(chan []byte)

	go func() {
		for line := range ch {
			c.RLock()
			cfg := c.cfg.Log
			for filename, v := range cfg {
				for pattern, s := range v.Match {
					if pattern.Match(line) {
						tmpls := strings.Split(s, ";")
						c.RUnlock()
						for _, tmpl := range tmpls {
							tmpl = strings.TrimSpace(tmpl)
							f := files[filename]
							if v.Timestamp {
								fmt.Fprintf(f, "%s ", time.Now().Format(time.Kitchen))
							}
							fmt.Fprintln(f, pattern.Expand(line, tmpl))
						}
						c.RLock()
					}
				}
			}
			c.RUnlock()
		}
	}()
	return ch, nil
}

func (c *Session) triggers(line []byte) {
	if c.triggersDisabled {
		return
	}
	for pattern := range c.cfg.Triggers {
		if pattern.Match(line) {
			// expand regexp capture groups
			template := c.cfg.Triggers[pattern]
			s := pattern.Expand(line, template)

			info.Fprintf(c.output, "[trigger: %s]\n", s)

			// expand aliases
			for _, sub := range c.expand(s) {
				fmt.Fprintf(c.output, "%s\n", sub)
				fmt.Fprintf(c.conn, "%s\n", sub)
			}
		}
	}
}

func (c *Session) gag(line []byte) bool {
	for _, pattern := range c.cfg.Gag {
		if pattern.Match(line) {
			return true
		}
	}
	return false
}

func (c *Session) replace(line []byte) ([]byte, bool) {
	ok := false
	for pattern, replace := range c.cfg.Replace {
		if pattern.Match(line) {
			ok = true
			line = []byte(replace.Color.Sprint(replace.With))
		}
	}
	return line, ok
}

func (c *Session) highlight(line []byte) ([]byte, bool) {
	if len(line) == 0 {
		return line, false
	}

	ok := false
	for pattern, color := range c.cfg.Highlight {
		if pattern.Match(line) {
			ok = true
			line = pattern.Color(line, color)
		}
	}
	return line, ok
}

func (c *Session) isPrompt(line []byte) bool {
	if c.cfg.Prompt == "" {
		return false
	}
	return c.cfg.Prompt.Match(line)
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

		if len(s) > 0 {
			switch s[0] {
			// shell command
			case '!':
				fmt.Fprintln(c.output, s)
				c.sys(s[1:])
				c.conn.Write([]byte("\n")) // get a fresh prompt
				continue

			// client command
			case '/':
				fields := strings.Fields(s)
				if f, ok := commands[fields[0]]; ok {
					fmt.Fprintln(c.output, s)
					f(c, fields[1:]...)
					c.conn.Write([]byte("\n")) // get a fresh prompt
					continue
				}
			}
		}

		c.RLock()
		expanded := c.expand(s)
		c.RUnlock()
		for _, sub := range expanded {
			if len(sub) > 0 {
				c.history = append(c.history, sub)
				if len(c.history) > 100 {
					c.history = c.history[1:]
				}
			}
			fmt.Fprintln(out, sub)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan input: %v", err)
	}
	return nil
}

func (c *Session) sys(command string) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = c.output
	cmd.Stderr = c.output
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(c.output, err)
	}
}

func (c *Session) expand(s string) []string {
	var out []string
	for _, sub := range strings.Split(s, ";") {
		sub := strings.TrimSpace(sub)
		if len(sub) == 0 {
			out = append(out, "")
			continue
		}

		// replace aliases
		words := strings.Fields(sub)
		if v, ok := c.cfg.Aliases[words[0]]; ok {
			info.Fprintf(c.output, "[alias: %s]\n", v)
			sub = v
		}

		// interpolate $vars in the command. $1, $*, are expanded as positional
		// parameters (useful for aliases) and other variables are expanded with
		// their configured values.
		var err error
		sub, err = interpolate.Interpolate(c.vars, words, sub)
		if err != nil {
			info.Fprintf(c.output, "[ERROR: %v]\n", err)
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
		c.Lock()
		dur := c.cfg.Tick.Duration
		time.AfterFunc(dur-10*time.Second, func() {
			alert.Fprintln(c.output, "10s until next tick.")
		})
		c.lastTick = t
		c.Unlock()
	}
}

func (c *Session) Login() error {
	if c.cfg.Login.Name != "" {
		if _, err := fmt.Fprintln(c.conn, c.cfg.Login.Name); err != nil {
			return err
		}
	}
	if c.cfg.Login.Password != "" {
		if _, err := fmt.Fprintln(c.conn, c.cfg.Login.Password); err != nil {
			return err
		}
	}
	return nil
}
