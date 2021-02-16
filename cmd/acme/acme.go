package main

import (
	"fmt"

	"9fans.net/go/acme"
)

type bodyWriter struct {
	*acme.Win
}

func (w bodyWriter) Write(p []byte) (int, error) {
	return w.Win.Write("body", p)
}

func commands(w *acme.Win) <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			switch e.C2 {
			case 'x', 'X': // execute
				if string(e.Text) == "Del" {
					close(c)
					w.Ctl("delete")
				}
				c <- string(e.Text)
			case 'l', 'L': // look
				w.Ctl("clean")
				c <- fmt.Sprintf("look %s", e.Text)
			}
		}
		w.CloseFiles()
		close(c)
	}()
	return c
}
