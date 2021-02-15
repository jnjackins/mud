package mud

import (
	"regexp"
	"sync"
)

type pattern string

var (
	patternLock sync.RWMutex
	patterns    map[pattern]*regexp.Regexp
)

func init() {
	patterns = make(map[pattern]*regexp.Regexp)
}

func (p pattern) match(s []byte) bool {
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

func (p pattern) expand(content, template string) string {
	patternLock.RLock()
	re := patterns[p]
	patternLock.RUnlock()

	var result []byte
	for _, submatches := range re.FindAllStringSubmatchIndex(content, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		result = re.ExpandString(result, template, content, submatches)
	}
	return string(result)
}
