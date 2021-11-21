package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jnjackins/mud"
	"github.com/jnjackins/mud/telnet"
)

type client struct {
	sessions map[string]*Session
	main     *Session
}

func main() {
	log.SetFlags(0)

	serve := flag.Bool("serve", true, "Run session server")
	login := flag.Bool("login", true, "Log in automatically")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s prefix:path ...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	c := &client{
		sessions: make(map[string]*Session),
	}
	for _, arg := range flag.Args() {
		parts := strings.Split(arg, ":")
		if len(parts) != 2 {
			log.Fatalf("bad session %q", arg)
		}
		prefix := parts[0]
		if prefix == "" {
			log.Fatalf("empty session prefix")
		}
		path := parts[1]

		sess, err := c.startSession(prefix, path, *serve, *login)
		if err != nil {
			log.Fatal(err)
		}
		defer sess.Close()
	}

	c.input()
}

func (c *client) startSession(prefix, path string, serve, login bool) (*Session, error) {
	cfg, err := mud.UnmarshalConfig(path + "/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	input, output, err := setupDir(path)
	if err != nil {
		return nil, err
	}

	log.Printf("%s: connecting", path)
	conn, err := telnet.Dial("tcp", cfg.Address)
	if err != nil {
		return nil, err
	}

	sess := &Session{
		prefix: prefix,
		path:   path,

		conn:   conn,
		input:  input,
		output: output,

		vars:            make(mapvars),
		oneTimeTriggers: make(map[mud.Pattern]string),
	}
	sess.SetConfig(cfg)

	if serve {
		go func() {
			if err := c.serve(sess, login); err != nil {
				log.Fatalf("serve: %v", err)
			}
		}()
	}

	sess.startCompleter()

	c.sessions[prefix] = sess
	if c.main == nil {
		c.main = sess
	}

	return sess, nil
}

func setupDir(path string) (input, output pipe, err error) {
	inputPath := filepath.Join(path, "/in")
	outputPath := filepath.Join(path, "/out")

	os.Mkdir(path, 0777)

	// ensure the input pipe exists
	var fi os.FileInfo
	fi, err = os.Stat(inputPath)
	if err == nil && fi.Mode()&os.ModeNamedPipe == 0 {
		err = fmt.Errorf("%s exists and is not named pipe", inputPath)
	} else if os.IsNotExist(err) {
		if err = syscall.Mkfifo(inputPath, 0666); err != nil {
			err = fmt.Errorf("mkfifo: %v", err)
		}
	}
	if err != nil {
		return
	}

	// hack: open read/write to avoid blocking waiting for writer
	input.r, err = os.OpenFile(inputPath, syscall.O_RDWR, 0)
	if err != nil {
		err = fmt.Errorf("open input reader: %v", err)
		return
	}
	input.w, err = os.OpenFile(inputPath, os.O_WRONLY, 0)
	if err != nil {
		err = fmt.Errorf("open input writer: %v", err)
		return
	}

	output.w, err = os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		err = fmt.Errorf("open output writer: %v", err)
		return
	}
	output.r, err = os.Open(outputPath)
	if err != nil {
		err = fmt.Errorf("open output reader: %v", err)
		return
	}
	output.r.(io.ReadSeeker).Seek(0, io.SeekEnd)

	return
}
