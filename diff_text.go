package main

import (
	"strconv"
	"strings"
)

func diffText(line string) string {
	line = strings.TrimSuffix(line, "\r")
	var out strings.Builder
	column := 0
	for _, r := range line {
		if r == '\t' {
			spaces := 8 - column%8
			out.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		} else {
			out.WriteRune(r)
			column += runeWidth(r)
		}
	}
	return safeText(out.String())
}

func diffNewLine(line string) (int, bool) {
	if !strings.HasPrefix(line, "@@ ") {
		return 0, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, false
	}
	start, _, _ := strings.Cut(fields[2][1:], ",")
	n, err := strconv.Atoi(start)
	return n, err == nil
}

// Locate the live edit in the HEAD diff by its current-file line, not text that may repeat.
func previewOffset(head, burst string) int {
	target, position := 0, 0
	for _, line := range strings.Split(burst, "\n") {
		if start, ok := diffNewLine(line); ok {
			position = start
			continue
		}
		if position == 0 || len(line) == 0 || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if line[0] == '+' {
			target = position
			break
		}
		if line[0] == '-' && target == 0 {
			target = position
		}
		if line[0] == ' ' {
			position++
		}
	}
	if target == 0 {
		return 0
	}
	position = 0
	for i, line := range strings.Split(head, "\n") {
		if start, ok := diffNewLine(line); ok {
			position = start
			continue
		}
		if position == 0 || len(line) == 0 {
			continue
		}
		if position >= target {
			return max(0, i-3)
		}
		if line[0] == '+' || line[0] == ' ' {
			position++
		}
	}
	return 0
}
