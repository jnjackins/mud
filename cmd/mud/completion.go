// TODO: also autocomplete from file of keywords (charm names, clan members, etc)
package main

import (
	"bufio"
	"io"
	"log"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fvbock/trie"
)

func (sess *Session) startCompleter() {
	br := bufio.NewReader(sess.output)

	sess.words = trie.NewTrie()
	sess.expireQueue = make(chan string, 200)

	go func() {
		var err error
		var r rune
		var sb strings.Builder
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
			if unicode.IsLetter(r) || r == '-' {
				sb.WriteRune(r)
			} else {
				if sb.Len() > 3 {
					word := strings.ToLower(sb.String())
					sess.words.Add(word)

					// queue of words added to the trie. When it's full, start
					// deleting the oldest members of the queue from the trie.
					select {
					case sess.expireQueue <- word:
					default:
						// expireQueue is full
						sess.Lock()
						sess.words.Delete(<-sess.expireQueue)
						sess.Unlock()
						sess.expireQueue <- word
					}
				}
				sb.Reset()
			}
		}
	}()

	return
}

func (sess *Session) complete(line string, pos int) (head string, completions []string, tail string) {
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
		sess.RLock()
		completions = sess.words.PrefixMembersList(partial)
		sess.RUnlock()
	} else {
		// otherwise, complete with aliases / abilities
		sess.Lock()
		cfg := sess.cfg
		sess.Unlock()
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
