package mud

import (
	"regexp"
	"sync"
)

type Pattern string

var (
	patternLock sync.RWMutex
	patterns    map[Pattern]*regexp.Regexp
)

func init() {
	patterns = make(map[Pattern]*regexp.Regexp)
}

func (p Pattern) Match(s []byte) bool {
	patternLock.RLock()
	re, ok := patterns[p]
	patternLock.RUnlock()

	if ok {
		return re.Match(s)
	}

	re = regexp.MustCompile(string(p))
	patternLock.Lock()
	patterns[p] = re
	patternLock.Unlock()
	return re.Match(s)
}

func (p Pattern) Expand(content []byte, template string) string {
	patternLock.RLock()
	re := patterns[p]
	patternLock.RUnlock()

	var result []byte
	for _, submatch := range re.FindAllSubmatchIndex(content, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		result = re.Expand(result, []byte(template), content, submatch)
	}
	return string(result)
}

func (p Pattern) Color(content []byte, color *Color) []byte {
	patternLock.RLock()
	re := patterns[p]
	patternLock.RUnlock()

	s := string(content)
	for _, submatch := range re.FindAllStringSubmatchIndex(s, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		colored := color.Sprint(string(s[submatch[0]:submatch[1]]))
		s = s[:submatch[0]] + colored + s[submatch[1]:]
	}
	return []byte(s)
}
