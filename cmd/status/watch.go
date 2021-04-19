package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jnjackins/mud/tail"
)

type event struct {
	name      string
	status    status
	charges   int
	remaining time.Duration
}

type status uint

const (
	down status = iota
	up
	on
	off
	mem
	cast
)

func watch(dir string) chan event {
	events := make(chan event)

	go func() {
		f, err := os.Open(dir + "/status.log")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		f.Seek(0, io.SeekEnd)

		statusPattern := regexp.MustCompile("^([a-z ]+) (up|down|on|off|for [0-9]+[smh]+)$")
		memPattern := regexp.MustCompile("^mem (.+)$")
		castPattern := regexp.MustCompile("^cast (.+)$")
		scanner := bufio.NewScanner(tail.New(context.Background(), f, 100*time.Millisecond))
		for scanner.Scan() {
			if memPattern.Match(scanner.Bytes()) {
				args := strings.Fields(scanner.Text())[1:]
				for _, arg := range args {
					parts := strings.Split(arg, ":")
					if len(parts) != 2 {
						continue
					}
					spell := parts[0]
					charges, err := strconv.Atoi(parts[1])
					if err != nil {
						continue
					}
					events <- event{name: spell, status: mem, charges: charges}
				}
			} else if castPattern.Match(scanner.Bytes()) {
				fields := strings.Fields(scanner.Text())
				spell := fields[1]
				events <- event{name: spell, status: cast}
			} else if statusPattern.Match(scanner.Bytes()) {
				matches := statusPattern.FindStringSubmatch(scanner.Text())
				name := strings.ToLower(matches[1])
				fields := strings.Fields(matches[2])
				switch fields[0] {
				case "up":
					events <- event{name: name, status: up}
				case "down":
					events <- event{name: name, status: down}

				case "on":
					events <- event{name: name, status: on}

				case "off":
					events <- event{name: name, status: off}

				case "for":
					dur, err := time.ParseDuration(fields[1])
					if err != nil {
						continue
					}
					events <- event{name: name, status: on, remaining: dur}
					go func(name string, dur time.Duration) {
						end := time.Now().Add(dur)
						tick := time.NewTicker(1 * time.Second)
						for range tick.C {
							remaining := time.Until(end)
							if remaining < 0 {
								break
							}
							events <- event{name: name, status: on, remaining: remaining}
						}
						tick.Stop()
						events <- event{name: name, status: off}
					}(name, dur)
					continue
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}()

	return events
}
