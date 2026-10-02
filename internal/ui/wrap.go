package ui

import "strings"

// WrapText breaks text into lines no wider than maxW, measured by width
// (usually Font.TextWidth). Newlines force a break, and a blank line
// survives as "". Words wider than maxW are split between characters.
func WrapText(text string, maxW float32, width func(string) float32) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if width(try) <= maxW {
				line = try
				continue
			}
			if line != "" {
				lines = append(lines, line)
			}
			line = word
			for width(line) > maxW && len([]rune(line)) > 1 {
				runes := []rune(line)
				cut := len(runes) - 1
				for cut > 1 && width(string(runes[:cut])) > maxW {
					cut--
				}
				lines = append(lines, string(runes[:cut]))
				line = string(runes[cut:])
			}
		}
		lines = append(lines, line)
	}
	return lines
}
