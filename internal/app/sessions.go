package app

import (
	"errors"
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
	total               repositoryLineCount
	modifications       map[string]int
	store               *activityStore
	activityErr         error
}

func (s repositorySession) newMonitor() *monitor {
	m := newMonitor(s.root, s.source, s.initial, s.events, s.scanErr == nil)
	m.store, m.storageErr = s.store, s.activityErr
	for p, n := range s.modifications {
		m.modifications[p] = n
	}
	return m
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
		s.initial, s.scanErr = source.Scan(watch.AllDepth)
		var initialErr error
		if s.scanErr == nil {
			s.events, initialErr = source.InitialChanges(s.initial, time.Now())
			n, known, totalErr := source.LineTotal()
			if totalErr == nil {
				s.total = repositoryLineCount{lines: n, known: known}
			}
		}
		s.store, s.activityErr = openActivityStore(root)
		if s.activityErr == nil {
			var dirty map[string]string
			if s.scanErr == nil && initialErr == nil {
				dirty = make(map[string]string)
				for _, e := range s.events {
					if !e.IsDir {
						dirty[e.Path] = e.Diff
					}
				}
			}
			s.modifications, s.activityErr = s.store.apply(dirty, nil, nil)
		}
		s.activityErr = errors.Join(initialErr, s.activityErr)
		sessions = append(sessions, s)
	}
	return sessions, nil
}
func seedSessions(u *ui, sessions []repositorySession) {
	u.repositoryLabels = make(map[string]string)
	u.repositoryTotals = make(map[string]repositoryLineCount, len(sessions))
	for _, s := range sessions {
		u.repositoryTotals[s.prefix] = s.total
		if s.prefix != "" {
			u.repositoryLabels[s.prefix] = s.label
		}
		if s.scanErr != nil {
			continue
		}
		up := namespaceUpdate(s.prefix, scanUpdate{snapshot: s.initial, events: s.events, baseline: true, modifications: s.modifications})
		u.applyScanUpdate(up)
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
