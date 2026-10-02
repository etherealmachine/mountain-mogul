package ui

import (
	"reflect"
	"testing"
)

func TestWrapText(t *testing.T) {
	w := func(s string) float32 { return float32(len(s)) }
	cases := []struct {
		text string
		maxW float32
		want []string
	}{
		{"the quick brown fox", 10, []string{"the quick", "brown fox"}},
		{"one\n\ntwo", 10, []string{"one", "", "two"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"", 10, []string{""}},
	}
	for _, c := range cases {
		if got := WrapText(c.text, c.maxW, w); !reflect.DeepEqual(got, c.want) {
			t.Errorf("WrapText(%q, %v) = %q, want %q", c.text, c.maxW, got, c.want)
		}
	}
}
