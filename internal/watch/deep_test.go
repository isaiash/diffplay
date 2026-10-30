package watch

import "testing"

func TestScanUnlimitedAndExplicitDepth(t *testing.T) {
	root := t.TempDir()
	deep := "a/b/c/d/e/f/g/h/file"
	put(t, root, deep, "deep\n")
	for _, depth := range []int{AllDepth, 9, 99} {
		s, err := Scan(root, depth)
		if err != nil {
			t.Fatal(err)
		}
		if string(s[deep].Content) != "deep\n" {
			t.Errorf("depth %d omitted deep file", depth)
		}
	}
	for _, depth := range []int{0, 5, 8} {
		s, err := Scan(root, depth)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := s[deep]; ok {
			t.Errorf("depth %d exceeded explicit bound", depth)
		}
	}
}
