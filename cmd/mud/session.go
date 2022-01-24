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
	triggersDisabled bool
	cancelTimers     context.CancelFunc
	oneTimeTriggers  map[mud.Pattern]string

	// tab completion
	words       *trie.Trie
	expireQueue chan string
}

func (s *Session) Close() error {
	s.input.Close()
	s.output.Close()
	return nil
}

func (s *Session) Start(login bool) error {
	errors := make(chan error)

	go func() {
		errors <- s.receive()
	}()

	go func() {
		errors <- s.send()
	}()

	if login {
		go func() {
			if err := s.Login(); err != nil {
				errors <- err
			}
		}()
	}

	return <-errors
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func (c *Session) receive() error {
	// mud doesn't send a newline after the prompt; we rigged the telnet reader
	// to send \x04 (EOT) instead.
	var eol bool
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexAny(data, "\x04\n"); i >= 0 {
			if data[i] == '\n' {
				eol = true
			} else {
				eol = false
			}
			return i + 1, dropCR(data[0:i]), nil
		}
		if atEOF {
			return len(data), dropCR(data), nil
		}
		// Request more data.
		return 0, nil, nil
	}

	scanner := bufio.NewScanner(c.conn)
	scanner.Split(split)

	logch, err := c.startLogWriter()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		logch <- line

		c.RLock()
		if s, ok := c.replace(line); ok {
			line = s
		} else if s, ok := c.highlight(line); ok {
			line = s
		}
		if !c.gag(line) {
			fmt.Fprint(c.output, string(line))
			if eol {
				fmt.Fprintln(c.output)
			}
		}
		c.triggers(line)
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
	f := func(line []byte, m map[mud.Pattern]string, oneTime bool) {
		if c.triggersDisabled {
			return
		}
		for pattern := range m {
			if pattern.Match(line) {
				template := m[pattern]

				if oneTime {
					delete(m, pattern)
				}

				// expand regexp capture groups
				s := pattern.Expand(line, template)

				info.Fprintf(c.output, "[trigger: %s]\n", s)

				// expand aliases
				for _, sub := range c.expand(s) {
					c.RUnlock()
					ok := c.command(sub)
					c.RLock()
					if !ok {
						fmt.Fprintln(c.conn, sub)
					}
				}
			}
		}
	}

	f(line, c.cfg.Triggers, false)   // permanently configured triggers
	f(line, c.oneTimeTriggers, true) // ad-hoc one-time triggers
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
			sprint := fmt.Sprint
			if replace.Color != nil {
				sprint = replace.Color.Sprint
			}
			line = []byte(sprint(pattern.Expand(line, replace.With)))
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
	scanner := bufio.NewScanner(c.input)
	for scanner.Scan() {
		s := scanner.Text()

		c.RLock()
		expanded := c.expand(s)
		c.RUnlock()

		needPrompt := false
		for _, sub := range expanded {
			quiet := false
			if len(sub) > 0 {
				c.history = append(c.history, sub)
				if len(c.history) > 100 {
					c.history = c.history[1:]
				}
				if sub[0] == '@' {
					quiet = true
					sub = sub[1:]
				}
			}
			if c.command(sub) {
				if !quiet {
					needPrompt = true
				}
			} else {
				fmt.Fprintln(c.conn, sub)
			}
			if !quiet {
				fmt.Fprintln(c.output, sub)
			}
		}
		if needPrompt {
			fmt.Fprintln(c.conn)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan input: %v", err)
	}
	return nil
}

func (c *Session) command(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[0] {
	// shell command
	case '!':
		c.sys(s[1:])
		break

	// client command
	case '/':
		fields, err := splitFields(s)
		if err != nil {
			fmt.Fprintln(c.output, err)
			break
		}
		if f, ok := commands[fields[0]]; ok {
			f(c, fields[1:]...)
			break
		}

	default:
		return false
	}

	return true
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
			// info.Fprintf(c.output, "[alias: %s]\n", v)
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
