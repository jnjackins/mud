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

	"github.com/peterh/liner"
	yaml "gopkg.in/yaml.v2"

	"github.com/jnjackins/mud/telnet"
)

var (
	configPath = flag.String("config", "", "Path to yaml file defining triggers and macros")
	scriptDir  = flag.String("pipe", "/tmp/mud", "Path to named script input and output pipes")
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

	c.Lock()
	defer c.Unlock()

	c.Macros = nil
	c.Triggers = nil

	if err := yaml.Unmarshal(buf, c); err != nil {
		return err
	}

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

	config := new(config)

	if *configPath != "" {
		// load config once synchronously, then start async worker
		fi, err := os.Stat(*configPath)
		if err != nil {
			log.Fatal(err)
		}
		mtime := fi.ModTime()
		if err := config.reload(); err != nil {
			log.Fatal(err)
		}
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

	// provide in and out pipes for scripting
	var pipeWriter io.Writer
	if *scriptDir != "" {
		var err error
		pipeWriter, err = scriptWorker(conn, config, *scriptDir)
		if err != nil {
			log.Fatal(err)
		}
	}

	errors := make(chan error)

	tickCh := make(chan time.Time)

	// MUD response reader
	go func() {
		outputs := []io.Writer{os.Stdout, pipeWriter}
		errors <- read(conn, outputs, config, tickCh)
	}()

	// primary input worker
	prompt := liner.NewLiner()
	go func() {
		errors <- write(conn, config, prompt)
	}()

	// tick worker
	go tick(conn, config, tickCh)

	// login automatically, if possible
	login(conn, config)

	err = <-errors

	prompt.Close()

	if err != nil {
		log.Fatal(err)
	}
}

func read(conn *telnet.Conn, outputs []io.Writer, config *config, tick chan<- time.Time) error {
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

			// force the prompt to refresh
			syscall.Kill(0, syscall.SIGWINCH)
		}

		// write to all outputs
		for _, w := range outputs {
			w.Write(buf[:n])
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
				break
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

		// first, check if it's a client command
		if f, ok := commands[s]; ok {
			f()
			conn.Write([]byte("\n"))
			continue
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
