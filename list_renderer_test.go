package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

var listANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func plainList(frame string) string { return listANSI.ReplaceAllString(frame, "") }

func TestListZeroCountsAreBlank(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("empty", "created", false, "", 0, 0, true, time.Now())
	frame := plainList(u.renderList(80, 24, time.Now()))
	if strings.Contains(frame, "+0") || strings.Contains(frame, "-0") {
		t.Fatalf("zero counts must be blank: %q", frame)
	}
	u.recordChange("empty", "modified", false, "binary", 0, 0, false, time.Now())
	frame = plainList(u.renderList(80, 24, time.Now()))
	if !strings.Contains(frame, "+?") || !strings.Contains(frame, "-?") {
		t.Fatal("unknown counts must remain explicit")
	}
}

func TestListHistogramShrinksWithTerminalAndPreservesPreview(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	u.recordChange("x", "modified", false, "+preview", 8, 4, true, now)
	for _, width := range []int{20, 30, 40, 60, 100, 120, 180} {
		frame := plainList(u.renderList(width, 24, now))
		lines := strings.Split(frame, "\r\n")
		row := lines[2]
		if !strings.Contains(row, "│") || !strings.Contains(row, "x") {
			t.Errorf("width %d lost histogram or path: %q", width, row)
		}
		for _, line := range lines {
			if textWidth(line) > width {
				t.Errorf("width %d overflow: %q", width, line)
			}
		}
		if width >= 120 && !strings.Contains(frame, "+preview") {
			t.Errorf("width %d lost diff preview", width)
		}
	}
}

func TestListDivergingHistogram(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	u.recordChange("folder/big", "modified", false, "", 16, 8, true, now)
	u.recordChange("small", "created", false, "", 1, 1, true, now)
	lines := strings.Split(plainList(u.renderList(100, 24, now)), "\r\n")
	folder, big, small := []rune(lines[2]), []rune(lines[3]), []rune(lines[4])
	center := -1
	for i, r := range big {
		if r == '│' {
			center = i
			break
		}
	}
	if center < 0 {
		t.Fatalf("missing centered histogram: %q", lines[4])
	}
	if big[center+1] != '█' || big[center-1] != '█' {
		t.Fatalf("bars must diverge: %q", lines[4])
	}
	if folder[center] != ' ' {
		t.Fatal("expanded aggregate histogram must be entirely blank")
	}
	if small[center+1] != '▎' || small[center-1] != '▊' {
		t.Fatalf("fractional widths must preserve scale: %q", lines[5])
	}
	u.listKey("c")
	lines = strings.Split(plainList(u.renderList(100, 24, now)), "\r\n")
	if !strings.Contains(lines[2], "█│█") {
		t.Fatalf("collapsed folder must show aggregate: %q", lines[3])
	}
}

func backgroundBeforeName(t *testing.T, frame, name string) rgb {
	t.Helper()
	index := strings.Index(frame, name)
	if index < 0 {
		t.Fatalf("missing filename %q", name)
	}
	matches := regexp.MustCompile(`\x1b\[48;2;([0-9]+);([0-9]+);([0-9]+)m`).FindAllStringSubmatch(frame[:index], -1)
	if len(matches) == 0 {
		t.Fatal("missing background")
	}
	last := matches[len(matches)-1]
	var color rgb
	fmt.Sscan(last[1], &color.r)
	fmt.Sscan(last[2], &color.g)
	fmt.Sscan(last[3], &color.b)
	return color
}

func TestListFlashUsesBrightEventTintEvenWhenSelected(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		kind  string
		color rgb
	}{{"created", blue}, {"modified", green}, {"deleted", red}} {
		t.Run(tc.kind, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.recordChange("target", "created", false, "", 1, 0, true, now.Add(-time.Second))
			u.recordChange("target", tc.kind, false, "", 1, 1, true, now)
			want := blend(tc.color, white, 0.45)
			frame := u.renderList(100, 24, now)
			if got := backgroundBeforeName(t, frame, "target"); got != want {
				t.Fatalf("selected flash = %+v, want pastel %+v", got, want)
			}
			middle := backgroundBeforeName(t, u.renderList(100, 24, now.Add(burstDuration/2)), "target")
			stale := backgroundBeforeName(t, u.renderList(100, 24, now.Add(burstDuration)), "target")
			if middle.r >= want.r || middle.g >= want.g || middle.b >= want.b || middle.r <= stale.r || middle.g <= stale.g || middle.b <= stale.b {
				t.Fatalf("flash did not interpolate: %+v %+v %+v", want, middle, stale)
			}
		})
	}
	u := newUI(newGraph(), "/repo")
	u.recordChange("folder/target", "deleted", false, "", 0, 2, true, now)
	if got := backgroundBeforeName(t, u.renderList(100, 24, now), "folder/"); got != blend(red, white, 0.45) {
		t.Fatalf("ancestor must use descendant event: %+v", got)
	}
}

func TestListFlashKeepsFilenameReadable(t *testing.T) {
	now := time.Now()
	for _, kind := range []string{"created", "modified", "deleted"} {
		u := newUI(newGraph(), "/repo")
		u.recordChange("target", kind, false, "", 1, 1, true, now)
		frame := u.renderList(100, 24, now)
		prefix := frame[:strings.Index(frame, "target")]
		matches := regexp.MustCompile(`\x1b\[38;2;([0-9]+);([0-9]+);([0-9]+)m`).FindAllStringSubmatch(prefix, -1)
		last := matches[len(matches)-1]
		var color rgb
		fmt.Sscan(last[1], &color.r)
		fmt.Sscan(last[2], &color.g)
		fmt.Sscan(last[3], &color.b)
		if color.r > 80 || color.g > 80 || color.b > 80 {
			t.Errorf("%s name must use dark ink on bright pastel: %+v", kind, color)
		}
	}
}

func TestListOmitsRootAndKeepsFocusVisible(t *testing.T) {
	u := changedUI()
	rows := u.changeRows()
	if len(rows) != 5 || rows[0].id != "docs" || rows[0].depth != 1 {
		t.Fatalf("visible rows start with direct children: %+v", rows)
	}
	if u.listSelected != "docs" {
		t.Fatalf("initial focus = %q", u.listSelected)
	}
	u.listKey("D")
	u.listKey("D")
	if u.listSelected != "docs" {
		t.Fatalf("left escaped to invisible root: %q", u.listSelected)
	}
	u.listKey("c")
	if u.listSelected != "docs" || len(u.changeRows()) != 2 {
		t.Fatal("collapse all must focus first visible child")
	}
	frame := plainList(u.renderList(100, 25, time.Now()))
	lines := strings.Split(frame, "\r\n")
	if strings.Contains(lines[2], "├─") || strings.Contains(lines[2], "repo/") {
		t.Fatalf("extra root trunk: %q", lines[3])
	}
}
