package main

import (
	"fmt"
	"strings"
	"unicode"
)

func splitFields(s string) ([]string, error) {
	bracketed := false
	parseFailure := false
	fields := strings.FieldsFunc(s, func(r rune) bool {
		if r == '{' {
			if bracketed {
				parseFailure = true
				return false
			}
			bracketed = true
			return true
		}
		if r == '}' {
			if !bracketed {
				parseFailure = true
				return false
			}
			bracketed = false
			return true
		}
		if bracketed {
			return false
		}
		return unicode.IsSpace(r)
	})

	if parseFailure {
		return nil, fmt.Errorf("failed to parse fields")
	}

	return fields, nil
}
