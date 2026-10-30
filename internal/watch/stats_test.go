package watch

import (
	"strings"
	"testing"
	"time"
)

func assertStats(t *testing.T, event Event, added, removed int, known bool) {
	t.Helper()
	if event.Added != added || event.Removed != removed || event.StatsKnown != known {
		t.Errorf("%s stats = (+%d, -%d, known=%v), want (+%d, -%d, known=%v)", event.Kind, event.Added, event.Removed, event.StatsKnown, added, removed, known)
	}
}

func TestDeletedLineStats(t *testing.T) {
	for _, tc := range []struct {
		text  string
		count int
	}{
		{"", 0},
		{"one\ntwo\n", 2},
		{"one\ntwo", 2},
		{strings.Repeat("line\n", 3000), 3000},
	} {
		events := Changes(Snapshot{"f": {Content: []byte(tc.text), Size: int64(len(tc.text))}}, nil, time.Time{})
		if len(events) != 1 {
			t.Fatalf("events = %v", events)
		}
		assertStats(t, events[0], 0, tc.count, true)
	}
}

func TestModifiedLineStats(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		added, removed      int
	}{
		{"replacement", "keep\nold\nend\n", "keep\nnew\nextra\nend\n", 2, 1},
		{"header-like source", "--old\n---old\nkeep\n", "++new\n+++new\nkeep\n", 2, 2},
		{"alignment", "a\nb\nc\nd\n", "b\nc\ne\n", 1, 2},
		{"unterminated", "old", "new", 1, 1},
		{"newline only", "same", "same\n", 1, 1},
		{"empty to text", "", "new", 1, 0},
		{"text to empty", "old", "", 0, 1},
		{"separate hunks", "old\n" + strings.Repeat("context\n", 20) + "old\n", "new\n" + strings.Repeat("context\n", 20) + "new\n", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := Changes(Snapshot{"f": {Content: []byte(tc.before)}}, Snapshot{"f": {Content: []byte(tc.after)}}, time.Time{})
			if len(events) != 1 {
				t.Fatalf("events = %v", events)
			}
			assertStats(t, events[0], tc.added, tc.removed, true)
		})
	}
}

func TestUnavailableLineStats(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, after Entry
	}{
		{"binary", Entry{Size: 2, Hash: [32]byte{1}}, Entry{Size: 2, Hash: [32]byte{2}}},
		{"truncated", Entry{Content: []byte("old"), Truncated: true}, Entry{Content: []byte("new"), Truncated: true}},
		{"byte bound", Entry{Content: []byte(strings.Repeat("a", ContentLimit+1))}, Entry{Content: []byte(strings.Repeat("b", ContentLimit+1))}},
		{"line bound", Entry{Content: []byte(strings.Repeat("a\n", 2001))}, Entry{Content: []byte(strings.Repeat("b\n", 2001))}},
		{"alignment bound", Entry{Content: []byte(strings.Repeat("a\n", 1000))}, Entry{Content: []byte(strings.Repeat("b\n", 1000))}},
		{"output bound", Entry{Content: []byte(strings.Repeat("a", DiffLimit))}, Entry{Content: []byte(strings.Repeat("b", DiffLimit))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := Changes(Snapshot{"f": tc.before}, Snapshot{"f": tc.after}, time.Time{})
			if len(events) != 1 {
				t.Fatalf("events = %v", events)
			}
			if tc.name == "binary" || tc.name == "truncated" {
				assertStats(t, events[0], 0, 0, false)
			} else {
				assertStats(t, events[0], len(lines(tc.after.Content)), len(lines(tc.before.Content)), true)
			}
			// Complete captured creations/deletions need no bounded alignment or output.
			if tc.name != "binary" && tc.name != "truncated" {
				return
			}
			for _, pair := range [][2]Snapshot{{nil, {"f": tc.after}}, {{"f": tc.before}, nil}} {
				events = Changes(pair[0], pair[1], time.Time{})
				if len(events) != 1 {
					t.Fatalf("events = %v", events)
				}
				assertStats(t, events[0], 0, 0, false)
			}
		})
	}
}

func TestMetadataLineStats(t *testing.T) {
	for _, text := range []string{"", "same\n", strings.Repeat("same\n", 3000)} {
		a := Entry{Content: []byte(text), Size: int64(len(text)), ModTime: time.Unix(1, 0)}
		b := a
		b.ModTime = time.Unix(2, 0)
		events := Changes(Snapshot{"f": a}, Snapshot{"f": b}, time.Time{})
		if len(events) != 1 {
			t.Fatalf("events = %v", events)
		}
		assertStats(t, events[0], 0, 0, true)
	}
}

func TestDirectoryLineStats(t *testing.T) {
	for _, pair := range [][2]Snapshot{
		{nil, {"dir": {IsDir: true}}},
		{{"dir": {IsDir: true}}, nil},
		{{"dir": {Content: []byte("text\n")}}, {"dir": {IsDir: true}}},
		{{"dir": {IsDir: true}}, {"dir": {Content: []byte("text\n")}}},
	} {
		events := Changes(pair[0], pair[1], time.Time{})
		if len(events) != 1 {
			t.Fatalf("events = %v", events)
		}
		assertStats(t, events[0], 0, 0, true)
	}
}

func TestCreatedLineStats(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		count      int
	}{
		{"empty", "", 0},
		{"terminated", "one\ntwo\n", 2},
		{"unterminated", "one\ntwo", 2},
		{"blank", "\n", 1},
		{"large captured addition", strings.Repeat("line\n", 3000), 3000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := Changes(nil, Snapshot{"f": {Content: []byte(tc.text), Size: int64(len(tc.text))}}, time.Time{})
			if len(events) != 1 {
				t.Fatalf("events = %v", events)
			}
			assertStats(t, events[0], tc.count, 0, true)
		})
	}
}
