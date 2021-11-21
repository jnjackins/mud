package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSplitFields(t *testing.T) {
	tests := map[string]struct {
		input  string
		result []string
		err    bool
	}{
		"simple": {
			input:  "foo bar",
			result: []string{"foo", "bar"},
		},
		"bracketed": {
			input:  "{foo}{bar}",
			result: []string{"foo", "bar"},
		},
		"bracketed with spaces": {
			input:  "{foo} {bar} {baz}",
			result: []string{"foo", "bar", "baz"},
		},
		"bad input": {
			input: "{{foo}",
			err:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := splitFields(test.input)
			if err != nil != test.err {
				t.Errorf("error mismatch: got (err != nil) == %v, want %v", err != nil, test.err)
			}
			if diff := cmp.Diff(test.result, got); diff != "" {
				t.Errorf("result mismatch: %v", diff)
			}
		})
	}
}
