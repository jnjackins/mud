package main

import (
	"bufio"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jnjackins/mud/telnet"
)

func scriptWorker(conn *telnet.Conn, config *config, dir string) (io.Writer, error) {
	os.Mkdir(dir, 0755)

	in := filepath.Join(dir, "in")
	os.Remove(in)
	if err := syscall.Mkfifo(in, 0600); err != nil {
		return nil, err
	}
	log.Printf("opening input file for reading")
	inp, err := os.OpenFile(in, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return nil, err
	}

	out := filepath.Join(dir, "out")
	os.Remove(out)
	if err := syscall.Mkfifo(out, 0600); err != nil {
		return nil, err
	}
	log.Printf("opening output file for writing")
	outp, err := os.OpenFile(out, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return nil, err
	}

	go func() {
		for range time.Tick(1 * time.Second) {
			scanner := bufio.NewScanner(inp)
			for scanner.Scan() {
				s := scanner.Text()
				log.Println(s)
				for _, sub := range expand(s, config) {
					if _, err := conn.Write([]byte(sub + "\n")); err != nil {
						log.Println(err)
					}
					dur := 1*time.Second + time.Duration(rand.Intn(500))*time.Millisecond
					time.Sleep(dur)
				}
			}
			if err := scanner.Err(); err != nil {
				log.Println(err)
			}
		}
	}()
	return outp, nil
}
