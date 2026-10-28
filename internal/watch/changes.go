package watch

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Event struct {
	Path  string
	Kind  string
	IsDir bool
	Diff  string
	At    time.Time

	// Added and Removed count line edits for this event, not net changes
	// against a session baseline. They are meaningful only when StatsKnown.
	Added      int
	Removed    int
	StatsKnown bool
}

// Changes compares captured contents, not Git state. Renames are delete/create.
// Directory timestamps and sizes are ignored because child activity changes them.
func Changes(before, after Snapshot, now time.Time) []Event {
	paths := make(map[string]bool, len(before)+len(after))
	for p := range before {
		paths[p] = true
	}
	for p := range after {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	var events []Event
	for _, p := range sorted {
		a, aok := before[p]
		b, bok := after[p]
		kind := ""
		switch {
		case !aok:
			kind = "created"
		case !bok:
			kind = "deleted"
		case a.IsDir != b.IsDir:
			kind = "modified"
		case a.IsDir:
			continue
		case a.Size != b.Size || !a.ModTime.Equal(b.ModTime) || a.Hash != b.Hash || !bytes.Equal(a.Content, b.Content):
			kind = "modified"
		default:
			continue
		}
		e := b
		if !bok {
			e = a
		}
		text, added, removed, known := diff(p, a, b, aok, bok)
		event := Event{Path: p, Kind: kind, IsDir: e.IsDir, Diff: text, At: now, Added: added, Removed: removed, StatsKnown: known}
		// A complete creation/deletion needs no alignment, even when its
		// rendered diff exceeds the line or output bounds.
		if (!aok || !bok) && !e.IsDir && !e.Truncated && len(e.Content) <= ContentLimit && (e.Content != nil || e.Size == 0) {
			if !aok {
				event.Added = len(lines(e.Content))
			} else {
				event.Removed = len(lines(e.Content))
			}
			event.StatsKnown = true
		}
		events = append(events, event)
	}
	return events
}

// DiffLimit bounds output; quadratic line alignment is separately capped at
// one million cells. Large inputs get an explicit marker, never unbounded work.
const DiffLimit = 64 * 1024

type edit struct {
	kind byte
	text string
}

func diff(path string, a, b Entry, aok, bok bool) (text string, added, removed int, known bool) {
	if a.IsDir || b.IsDir {
		return "", 0, 0, true
	}
	oldName, newName := "a/"+path, "b/"+path
	if !aok {
		oldName = "/dev/null"
	}
	if !bok {
		newName = "/dev/null"
	}
	header := fmt.Sprintf("--- %s\n+++ %s\n", oldName, newName)
	if (aok && a.Content == nil && a.Size > 0) || (bok && b.Content == nil && b.Size > 0) {
		return header + "Binary files differ (content not captured)\n", 0, 0, false
	}
	if a.Truncated || b.Truncated {
		return header + "Diff omitted: captured content truncated\n", 0, 0, false
	}
	omitted := header + "Diff omitted: text exceeds bounded diff limits\n"
	if len(a.Content) > ContentLimit || len(b.Content) > ContentLimit {
		return omitted, 0, 0, false
	}
	old, new := lines(a.Content), lines(b.Content)
	if len(old) > 2000 || len(new) > 2000 || (len(old)+1)*(len(new)+1) > 1000000 {
		// Identical complete text proves a metadata-only change without
		// needing the bounded alignment or altering the existing diff marker.
		return omitted, 0, 0, bytes.Equal(a.Content, b.Content)
	}
	width := len(new) + 1
	table := make([]uint16, (len(old)+1)*width)
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(new) - 1; j >= 0; j-- {
			at := i*width + j
			if old[i] == new[j] {
				table[at] = table[(i+1)*width+j+1] + 1
			} else {
				table[at] = max(table[(i+1)*width+j], table[i*width+j+1])
			}
		}
	}
	var ops []edit
	for i, j := 0, 0; i < len(old) || j < len(new); {
		switch {
		case i < len(old) && j < len(new) && old[i] == new[j]:
			ops = append(ops, edit{' ', old[i]})
			i++
			j++
		case i < len(old) && (j == len(new) || table[(i+1)*width+j] >= table[i*width+j+1]):
			ops = append(ops, edit{'-', old[i]})
			removed++
			i++
		default:
			ops = append(ops, edit{'+', new[j]})
			added++
			j++
		}
	}
	var out strings.Builder
	out.WriteString(header)
	oldPos, newPos := 1, 1
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			oldPos++
			newPos++
			i++
			continue
		}
		begin := max(0, i-3)
		oldStart, newStart := oldPos-(i-begin), newPos-(i-begin)
		// Merge changes separated by at most six context lines, preventing
		// neighboring three-line context windows from overlapping.
		lastChange := i
		for k := i + 1; k < len(ops) && k <= lastChange+7; k++ {
			if ops[k].kind != ' ' {
				lastChange = k
			}
		}
		end := min(len(ops), lastChange+4)
		oldCount, newCount := 0, 0
		for _, op := range ops[begin:end] {
			if op.kind != '+' {
				oldCount++
			}
			if op.kind != '-' {
				newCount++
			}
		}
		if oldCount == 0 {
			oldStart--
		}
		if newCount == 0 {
			newStart--
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, op := range ops[begin:end] {
			out.WriteByte(op.kind)
			out.WriteString(op.text)
			if !strings.HasSuffix(op.text, "\n") {
				out.WriteString("\n\\ No newline at end of file\n")
			}
			if out.Len() > DiffLimit {
				return omitted, 0, 0, false
			}
		}
		for _, op := range ops[i:end] {
			if op.kind != '+' {
				oldPos++
			}
			if op.kind != '-' {
				newPos++
			}
		}
		i = end
	}
	if len(ops) == 0 || bytes.Equal(a.Content, b.Content) {
		return header + "Metadata or uncaptured content changed\n", 0, 0, true
	}
	return out.String(), added, removed, true
}
func lines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := strings.SplitAfter(string(b), "\n")
	if s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	return s
}
