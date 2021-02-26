package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jnjackins/mud"
	"github.com/jnjackins/mud/telnet"
)

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s dir ...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	var sessions []*mud.Session
	for _, dir := range flag.Args() {
		s, err := session(dir)
		if err != nil {
			log.Println(err)
			continue
		}
		sessions = append(sessions, s)
	}

	select {}
}

func session(dir string) (*mud.Session, error) {
	log := log.New(os.Stderr, dir+": ", log.LstdFlags)

	fin, fout, flog, err := setupDir(dir)
	if err != nil {
		return nil, err
	}

	log.Println("loading configuration")
	cfgPath := dir + "/config.yaml"
	cfg, err := mud.UnmarshalConfig(cfgPath)
	if err != nil {
		return nil, err
	}

	log.Println("connecting")
	conn, err := telnet.Dial("tcp", cfg.Address)
	if err != nil {
		return nil, err
	}

	s := mud.New(cfg, conn, fin, fout, flog)

	s.SetConfig(cfg)

	// Monitor config file for changes
	fi, err := os.Stat(cfgPath)
	if err != nil {
		return nil, err
	}
	mtime := fi.ModTime()
	go func() {
		for range time.Tick(5 * time.Second) {
			fi, err := os.Stat(cfgPath)
			if err != nil {
				log.Println(err)
				continue
			}
			if fi.ModTime().After(mtime) {
				cfg, err = mud.UnmarshalConfig(cfgPath)
				if err != nil {
					log.Println(err)
					continue
				}
				s.SetConfig(cfg)
				log.Println("configuration updated")
				mtime = fi.ModTime()
			}
		}
	}()

	return s, nil
}

func setupDir(path string) (fin io.Reader, fout, flog io.Writer, err error) {
	in := filepath.Join(path, "/in")

	os.Mkdir(path, 0777)

	// ensure the input pipe exists
	var fi os.FileInfo
	fi, err = os.Stat(in)
	if err == nil && fi.Mode()&os.ModeNamedPipe == 0 {
		err = fmt.Errorf("%s exists and is not named pipe", in)
	} else if os.IsNotExist(err) {
		if err = syscall.Mkfifo(in, 0666); err != nil {
			err = fmt.Errorf("mkfifo: %v", err)
		}
	}
	if err != nil {
		return
	}

	fin, err = os.Open(in)
	if err != nil {
		err = fmt.Errorf("open input: %v", err)
		return
	}
	fout, err = os.OpenFile(filepath.Join(path, "/out"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		err = fmt.Errorf("open out: %v", err)
		return
	}
	flog, err = os.OpenFile(filepath.Join(path, "/log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		err = fmt.Errorf("open log: %v", err)
		return
	}
	return
}
