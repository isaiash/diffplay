package app

import (
	"fmt"
	"strings"
	"testing"
)

func TestPickerFragmentedInputScrollingAndMouse(t *testing.T) {
	paths := make([]string, 30)
	for i := range paths {
		paths[i] = fmt.Sprintf("/workspace/repo-%02d", i)
	}
	p := newRepositoryPicker(paths)
	p.render(80, 10)
	p.input([]byte("\x1b["))
	if p.focus != 0 {
		t.Fatal("partial key must wait")
	}
	p.input([]byte("F"))
	p.render(80, 10)
	if p.focus != 29 || p.offset == 0 {
		t.Fatal("End must scroll to last repository")
	}
	p.input([]byte("a"))
	if len(p.selectedPaths()) != 30 {
		t.Fatal("select all failed")
	}
	p.input([]byte("c\x1b[H"))
	p.render(80, 10)
	p.input([]byte("\x1b[<35;10;"))
	p.input([]byte("5M"))
	if p.focus != 1 {
		t.Fatal("hover must focus repository row")
	}
	p.input([]byte("\x1b[<0;10;5M"))
	if !p.checked[1] {
		t.Fatal("click must toggle selection")
	}
	p.input([]byte("\x1b[<65;10;5M"))
	if p.focus <= 1 {
		t.Fatal("mouse wheel must move focus")
	}
	for _, size := range [][2]int{{1, 1}, {20, 8}, {100, 35}} {
		plain := plainList(p.render(size[0], size[1]))
		for _, row := range strings.Split(plain, "\r\n") {
			if textWidth(row) > size[0] {
				t.Fatal("picker overflow")
			}
		}
	}
}

func TestPickerSanitizesPaths(t *testing.T) {
	p := newRepositoryPicker([]string{"/repo/evil\x1b[2J\nname"})
	raw := p.render(80, 20)
	if strings.Contains(raw, "\x1b[2J") {
		t.Fatal("repository path injected terminal escape")
	}
	if p.selectedPaths()[0] != "/repo/evil\x1b[2J\nname" {
		t.Fatal("display sanitization must preserve actual selection identifier")
	}
}
