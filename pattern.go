package mud

import (
	"bytes"
	"fmt"
	"regexp"
	"sync"
)

type Pattern string

type result struct {
	*regexp.Regexp
	err error
}

var pcache struct {
	sync.RWMutex
	patterns map[Pattern]result
}

func init() {
	pcache.patterns = make(map[Pattern]result)
}

func (p Pattern) get() (*regexp.Regexp, error) {
	pcache.RLock()
	re, ok := pcache.patterns[p]
	pcache.RUnlock()

	if ok {
		return re.Regexp, re.err
	}

	compiled, err := regexp.Compile(string(p))
	pcache.Lock()
	pcache.patterns[p] = result{
		Regexp: compiled,
		err:    err,
	}
	pcache.Unlock()
	return compiled, err
}

func (p Pattern) Match(s []byte) bool {
	re, err := p.get()
	if err != nil {
		fmt.Println(err)
		return false
	}
	return re.Match(s)
}

func (p Pattern) Expand(content []byte, template string) string {
	re, err := p.get()
	if err != nil {
		fmt.Println(err)
		return ""
	}

	var result []byte
	for _, submatch := range re.FindAllSubmatchIndex(content, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		result = re.Expand(result, []byte(template), content, submatch)
	}
	return string(result)
}

func (p Pattern) Color(s []byte, color *Color) []byte {
	re, err := p.get()
	if err != nil {
		fmt.Println(err)
		return s
	}

	var parts [][]byte
	var prevIndex int
	for _, match := range re.FindAllIndex(s, -1) {
		prev := s[prevIndex:match[0]]
		if len(prev) > 0 {
			parts = append(parts, prev)
		}
		colored := []byte(color.Sprint(string(s[match[0]:match[1]])))
		parts = append(parts, colored)
		prevIndex = match[1]
	}
	final := s[prevIndex:]
	if len(final) > 0 {
		parts = append(parts, final)
	}
	return bytes.Join(parts, []byte{})
}
