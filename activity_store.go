package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type activityStore struct{ path string }

func openActivityStore(root string) (*activityStore, error) {
	canonical, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return nil, err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("activity storage: %w", err)
	}
	hash := sha256.Sum256([]byte(canonical))
	return &activityStore{path: filepath.Join(config, "diffplay", "activity", fmt.Sprintf("%x.json", hash))}, nil
}

// apply merges deltas under a process-shared lock; nil dirty means Git status is unavailable.
func (s *activityStore) apply(dirty map[string]string, increments map[string]int, resets map[string]bool) (counts map[string]int, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("activity storage %s (repair permissions or remove this file to reset): %w", s.path, err)
		}
	}()
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return nil, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	counts = make(map[string]int)
	data, err := os.ReadFile(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	exists := err == nil
	if exists {
		if err = json.Unmarshal(data, &counts); err != nil {
			return nil, err
		}
		if counts == nil {
			return nil, errors.New("invalid null activity counts")
		}
		for p, n := range counts {
			if p == "." || p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") || strings.ContainsRune(p, 0) || n < 0 {
				return nil, errors.New("invalid activity path or count")
			}
		}
	}
	changed := false
	for p := range resets {
		if _, ok := counts[p]; ok {
			delete(counts, p)
			changed = true
		}
	}
	for p, n := range increments {
		if n <= 0 || counts[p] > int(^uint(0)>>1)-n {
			return nil, errors.New("activity count overflow")
		}
		counts[p] += n
		changed = true
	}
	for p, n := range counts {
		_, isDirty := dirty[p]
		if n == 0 || dirty != nil && !isDirty {
			delete(counts, p)
			changed = true
		}
	}
	if !changed {
		return counts, nil
	}
	data, err = json.Marshal(counts)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".activity-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(file.Name(), s.path); err != nil {
		return nil, err
	}
	return counts, nil
}
