package main

import (
	"io"
	"log"
	"os"

	"github.com/peterh/liner"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) != 2 {
		log.Fatalf("usage: %v <path>\n", os.Args[0])
	}
	path := os.Args[1]

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	liner := liner.NewLiner()
	defer liner.Close()

	for {
		s, err := liner.Prompt("")
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Println(err)
		}
		if _, err := f.Write([]byte(s + "\n")); err != nil {
			log.Println(err)
		}
		if len(s) > 1 {
			liner.AppendHistory(s)
		}
	}
}
