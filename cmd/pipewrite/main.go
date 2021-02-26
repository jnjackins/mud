package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fvbock/trie"
	"github.com/jnjackins/mud"
	"github.com/peterh/liner"
)

type session struct {
	in  io.Writer
	out io.ReadSeeker
	cfg mud.Config
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Fatalf("usage: %v prefix:path ...\n", os.Args[0])
	}

	sessions := make(map[string]*session)
	var main *session

	for _, arg := range os.Args[1:] {
		parts := strings.Split(arg, ":")
		if len(parts) != 2 {
			log.Fatalf("bad session %q", arg)
		}
		prefix := parts[0]
		if prefix == "" {
			log.Fatalf("empty session prefix")
		}
		path := parts[1]

		cfg, err := mud.UnmarshalConfig(path + "/config.yaml")
		if err != nil {
			log.Fatalf("read config: %v", err)
		}

		in, err := os.OpenFile(path+"/in", os.O_WRONLY, 0)
		if err != nil {
			log.Fatal(err)
		}
		defer in.Close()

		out, err := os.Open(path + "/out")
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()

		s := &session{
			in:  in,
			out: out,
			cfg: cfg,
		}

		sessions[prefix] = s

		if main == nil {
			main = s
		}
	}

	l := liner.NewLiner()
	defer l.Close()

	l.SetWordCompleter(completeFrom(main.out, main.cfg))
	l.SetTabCompletionStyle(liner.TabPrints)

	input(l, main, sessions)
}

func completeFrom(r io.ReadSeeker, cfg mud.Config) liner.WordCompleter {
	r.Seek(0, io.SeekEnd)
	br := bufio.NewReader(r)
	words := trie.NewTrie()

	go func() {
		var err error
		var r rune
		var sb strings.Builder
		delchan := make(chan string, 100)
		for {
			r, _, err = br.ReadRune()
			if err != nil {
				if err == io.EOF {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				log.Printf("completer: %v", err)
				break
			}
			if unicode.IsLetter(r) {
				sb.WriteRune(r)
			} else {
				if sb.Len() > 3 {
					word := strings.ToLower(sb.String())
					words.Add(word)

					// queue of words added to the trie. When it's full, start
					// deleting the oldest members of the queue from the trie.
					select {
					case delchan <- word:
					default:
						// delchan is full
						words.Delete(<-delchan)
						delchan <- word
					}
				}
				sb.Reset()
			}
		}
	}()

	f := func(line string, pos int) (head string, completions []string, tail string) {
		tail = string(line[pos:])
		head = line[:pos]

		var completingArg bool

		var nn int
		for len(head) > 0 {
			r, n := utf8.DecodeLastRuneInString(head)
			if unicode.IsSpace(r) {
				completingArg = true
				break
			}
			head = head[:len(head)-n]
			nn += n
		}
		partial := strings.ToLower(line[pos-nn : pos])

		if completingArg {
			// if not first word, complete with words we've seen in output
			completions = words.PrefixMembersList(partial)
		} else {
			// otherwise, complete with aliases / abilities
			for s := range cfg.Aliases {
				if strings.HasPrefix(s, partial) {
					completions = append(completions, s)
				}
			}
			for s := range cfg.Abilities {
				if strings.HasPrefix(s, partial) {
					completions = append(completions, s)
				}
			}
		}

		// if only 1 match, indicate with a space
		if len(completions) == 1 {
			completions[0] = completions[0] + " "
		}

		return
	}

	return f
}

func toSlice(m map[string]struct{}) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

func input(l *liner.State, main *session, sessions map[string]*session) {
	for {
		s, err := l.Prompt(main.cfg.Login.Name + "> ")
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
		for _, cmd := range strings.Split(s, ";") {
			var inputs []io.Writer
			fields := strings.Fields(cmd)
			if len(fields) > 0 {
				for _, prefix := range strings.Split(fields[0], ",") {
					if sess, ok := sessions[prefix]; ok {
						inputs = append(inputs, sess.in)
					}
				}
			}
			if len(inputs) == 0 {
				inputs = append(inputs, main.in)
			} else {
				// remove prefixes before sending cmd
				cmd = strings.Join(fields[1:], " ")
			}
			w := io.MultiWriter(inputs...)
			if _, err := w.Write([]byte(cmd + "\n")); err != nil {
				log.Println(err)
			}
		}
		if len(s) > 1 {
			l.AppendHistory(s)
		}
	}
}
