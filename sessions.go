package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
	"github.com/isaiash/diffplay/internal/watch"
)

type repositorySession struct {
	prefix, label, root string
	source              repositorySource
	initial             watch.Snapshot
	events              []changeEvent
	scanErr             error
}

func loadSessions(base string, paths []string, grouped bool) ([]repositorySession, error) {
	sessions := make([]repositorySession, 0, len(paths))
	for i, root := range paths {
		source, err := gitrepo.Open(root)
		if err != nil {
			return nil, err
		}
		s := repositorySession{root: root, source: source}
		if grouped {
			s.prefix = groupKey(i)
			label, err := filepath.Rel(base, root)
			if err != nil {
				return nil, err
			}
			s.label = filepath.ToSlash(label)
		}
		s.initial, s.scanErr = source.Scan(5)
		if s.scanErr == nil {
			s.events, err = source.InitialChanges(s.initial, time.Now())
			if err != nil {
				return nil, err
			}
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
func seedSessions(u *ui, sessions []repositorySession) {
	u.repositoryLabels = make(map[string]string)
	for _, s := range sessions {
		if s.prefix != "" {
			u.repositoryLabels[s.prefix] = s.label
		}
		up := namespaceUpdate(s.prefix, scanUpdate{events: s.events})
		u.seedEvents(up.events)
	}
}
func sessionWarnings(sessions []repositorySession, failures map[string]error) string {
	var warnings []string
	for _, s := range sessions {
		if err := failures[s.root]; err != nil {
			label := s.label
			if label == "" {
				label = s.root
			}
			warnings = append(warnings, fmt.Sprintf("%s: %v", label, err))
		}
	}
	sort.Strings(warnings)
	return strings.Join(warnings, "; ")
}
