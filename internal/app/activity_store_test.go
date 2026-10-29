package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestActivityStoreCanonicalPrivateAndConcurrent(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	a, err := openActivityStore(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := openActivityStore(alias)
	if err != nil {
		t.Fatal(err)
	}
	if a.path != b.path || !strings.HasPrefix(a.path, config+string(os.PathSeparator)) || filepath.Ext(a.path) != ".json" {
		t.Fatalf("noncanonical or unsafe storage path: %s %s", a.path, b.path)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.apply(map[string]string{"a": "diff"}, map[string]int{"a": 1}, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	counts, err := b.apply(nil, nil, nil)
	if err != nil || counts["a"] != 20 {
		t.Fatalf("lost concurrent increments: %v %v", counts, err)
	}
	for _, p := range []string{a.path, a.path + ".lock"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("exposed activity file %s: %v", p, info.Mode())
		}
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 0 {
		t.Fatal("store wrote into repository")
	}
}

func TestActivityStoreCorruptionIsReportedWithoutOverwriting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	s, err := openActivityStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		t.Fatal(err)
	}
	invalid := []byte("{broken json")
	if err = os.WriteFile(s.path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.apply(nil, map[string]int{"a": 1}, nil); err == nil || !strings.Contains(err.Error(), s.path) || !strings.Contains(err.Error(), "reset") {
		t.Fatalf("unactionable corruption error: %v", err)
	}
	content, err := os.ReadFile(s.path)
	if err != nil || string(content) != string(invalid) {
		t.Fatal("corrupt state silently overwritten")
	}
}
