package main

import (
	"io"
	"log"
	"strings"
	"time"

	"github.com/peterh/liner"
)

func (c *client) input() {
	l := liner.NewLiner()
	defer l.Close()

	l.SetWordCompleter(c.main.complete)
	l.SetTabCompletionStyle(liner.TabCircular)

	for {
		s, err := l.Prompt(c.main.cfg.Login.Name + "> ")
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Println(err)
		}

		// for each semicolon-separated command, check the first word for
		// comma-separated prefixes, and send commands to all sessions specified
		// by the prefixes. If there are sessions specified, send to main.
		// example command: `a,b look; b jump`
		for n, cmd := range c.main.expand(s) {
			cmd = strings.TrimSpace(cmd)

			if sess, ok := c.sessions[cmd]; ok {
				// only prefix was sent: change main to given session
				c.main = sess
				l.SetWordCompleter(c.main.complete)
				continue
			}

			var inputs []io.Writer
			fields := strings.Fields(cmd)
			if len(fields) > 0 {
				for _, prefix := range strings.Split(fields[0], ",") {
					if sess, ok := c.sessions[prefix]; ok {
						inputs = append(inputs, sess.input)
					}
				}
			}
			if len(inputs) == 0 {
				inputs = append(inputs, c.main.input)
			} else {
				// remove prefixes before sending cmd
				cmd = strings.Join(fields[1:], " ")
			}
			w := io.MultiWriter(inputs...)

			// attempt to order string commands between sessions by inserting a
			// small delay
			go func(n int, cmd string) {
				time.Sleep(time.Duration(n) * 5 * time.Millisecond)
				if _, err := w.Write([]byte(cmd + "\n")); err != nil {
					log.Println(err)
				}
			}(n, cmd)
		}

		if len(s) > 1 {
			l.AppendHistory(s)
		}
	}
}
