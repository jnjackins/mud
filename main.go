package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/peterh/liner"
	yaml "gopkg.in/yaml.v2"

	"github.com/jnjackins/mud/telnet"
)

var (
	configPath = flag.String("config", "", "Path to yaml file defining triggers and macros")
	debug      = flag.Bool("debug", false, "Print debug information")
)

type config struct {
	sync.RWMutex
	Login    map[string]string
	Triggers map[string]string
	Macros   map[string]string
	Tick     struct {
		Duration time.Duration
		Match    []string
	}
}

func (c *config) reload() error {
	buf, err := ioutil.ReadFile(*configPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(buf, c); err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	return nil
}

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <address>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	// load config once synchronously, then start async workero
	config := new(config)
	fi, err := os.Stat(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	mtime := fi.ModTime()
	if err := config.reload(); err != nil {
		log.Fatal(err)
	}
	if *configPath != "" {
		go func() {
			for range time.Tick(5 * time.Second) {
				fi, err := os.Stat(*configPath)
				if err != nil {
					log.Println(err)
					break
				}
				if fi.ModTime().After(mtime) {
					if err := config.reload(); err != nil {
						log.Println(err)
						break
					}
					fmt.Println("[configuration updated]")
					mtime = fi.ModTime()
				}
			}
			fmt.Println("[disabling configuration updates]")
		}()
	}

	conn, err := telnet.Dial("tcp", flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	errors := make(chan error)

	tickCh := make(chan time.Time)

	go func() {
		errors <- read(conn, config, tickCh)
	}()

	prompt := liner.NewLiner()

	go func() {
		errors <- write(conn, config, prompt)
	}()

	go tick(conn, config, tickCh)

	login(conn, config)

	err = <-errors

	prompt.Close()

	if err != nil {
		log.Fatal(err)
	}
}

func read(conn *telnet.Conn, config *config, tick chan<- time.Time) error {
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if n < len(buf) {
			// we've read a screenful; add a newline
			// to allow for a prompt with line-editing.
			buf[n] = '\n'
			n++

			// hack: force the prompt to refresh
			syscall.Kill(0, syscall.SIGWINCH)
		}
		if _, err := os.Stdout.Write(buf[:n]); err != nil {
			return err
		}

		// execute triggers
		config.RLock()
		for k := range config.Triggers {
			if bytes.Contains(buf[:n], []byte(k)) {
				s := config.Triggers[k]
				fmt.Printf("[trigger: %s]\n", s)
				for _, sub := range expand(s, config) {
					if *debug {
						fmt.Printf("[%s]\n", sub)
					}
					if _, err := conn.Write([]byte(sub + "\n")); err != nil {
						return err
					}
				}

			}
		}

		// tick timer
		for _, s := range config.Tick.Match {
			if bytes.Contains(buf[:n], []byte(s)) {
				tick <- time.Now()
			}
		}
		config.RUnlock()
	}
}

func write(conn *telnet.Conn, config *config, prompt *liner.State) error {
	// keep the connection alive
	go func() {
		for range time.Tick(5 * time.Minute) {
			conn.Write([]byte("\n"))
		}
	}()

	for {
		s, err := prompt.Prompt("")
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if len(s) > 3 {
			prompt.AppendHistory(s)
		}

		for _, sub := range expand(s, config) {
			if *debug {
				fmt.Printf("[expanded: %s]\n", sub)
			}
			if _, err := conn.Write([]byte(sub + "\n")); err != nil {
				return err
			}
		}
	}
}

func expand(s string, config *config) []string {
	var out []string
	for _, sub := range strings.Split(s, ";") {
		sub := strings.TrimSpace(sub)

		// replace macros
		config.RLock()
		if v, ok := config.Macros[sub]; ok {
			fmt.Printf("[macro: %s]\n", v)
			sub = v
		}
		config.RUnlock()

		if strings.Contains(sub, ";") {
			out = append(out, expand(sub, config)...)
		} else {
			out = append(out, sub)
		}
	}
	return out
}

func login(conn *telnet.Conn, config *config) {
	if v, ok := config.Login["name"]; ok {
		conn.Write([]byte(v + "\n"))
	}
	if v, ok := config.Login["pass"]; ok {
		conn.Write([]byte(v + "\n"))
	}
}

func tick(conn *telnet.Conn, config *config, tick <-chan time.Time) {
	var lastTick time.Time
	for t := range tick {
		if *debug && !lastTick.IsZero() {
			fmt.Printf("%v since last tick", t.Sub(lastTick))
		}
		time.AfterFunc(config.Tick.Duration-10*time.Second, func() {
			color.HiMagenta("10s until next tick.")
		})
		lastTick = t
	}
}
