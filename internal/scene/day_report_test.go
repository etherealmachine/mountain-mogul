package scene

import "testing"

func TestFormatDollars(t *testing.T) {
	for n, want := range map[int]string{
		0:        "$0",
		999:      "$999",
		1000:     "$1,000",
		-2400:    "-$2,400",
		209269:   "$209,269",
		-1234567: "-$1,234,567",
	} {
		if got := formatDollars(n); got != want {
			t.Errorf("formatDollars(%d) = %q, want %q", n, got, want)
		}
	}
}
