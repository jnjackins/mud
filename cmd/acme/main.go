package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	sessions, err := loadSessions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	for _, sess := range sessions {
		if err := sess.start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	select {}
}
