package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
	"github.com/isaiash/diffplay/internal/watch"
	"golang.org/x/term"
)

const version = "0.6.0"

type changeEvent = watch.Event

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "diffplay:", err)
		os.Exit(1)
	}
}
func applySnapshot(g *graph, s watch.Snapshot, events []changeEvent, now time.Time) {
	// The full snapshot is only a monitoring baseline, not the graph's node set.
	// Keep changed paths and the minimum directory context needed to group them.
	for _, e := range events {
		if e.Path == "." {
			continue
		}
		var parents []string
		for p := path.Dir(e.Path); p != "."; p = path.Dir(p) {
			parents = append(parents, p)
		}
		for i := len(parents) - 1; i >= 0; i-- {
			p := parents[i]
			if g.nodes[p] == nil {
				g.upsert(p, path.Dir(p), true)
			}
		}
		g.upsert(e.Path, path.Dir(e.Path), e.IsDir)
		g.mark(e.Path, e.Kind, e.Diff, e.At)
	}
	g.expire(now)
}
func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("diffplay", flag.ContinueOnError)
	flags.SetOutput(out)
	interval := flags.Duration("interval", 400*time.Millisecond, "filesystem polling interval (minimum 50ms)")
	snapshot := flags.Bool("snapshot", false, "print a JSON file listing and exit; no terminal required")
	showVersion := flags.Bool("version", false, "print version and exit")
	view := flags.String("view", "list", "initial view: graph or list (Tab switches views)")
	flags.Usage = func() {
		fmt.Fprintln(out, "Diffplay\n\nUsage: diffplay [options] [directory]\n\nLive repository activity inside your terminal: graph and sorted change tree.\nDefaults to the current directory; Git-ignored paths are excluded; depth <= 5.\nOutside Git, discover repositories within 5 levels and select one or more.\nStartup Git changes are loaded quietly; only subsequent live changes flash.\n\nTab: switch graph/list. List: arrows navigate, right expands, left collapses,\nclick toggles folders, hover selects, wheel scrolls, a/c expands/collapses all.\nGraph: hover diff, drag nodes, wheel/+/- zoom, n/p selects nodes.\nq: quit. Counts start from Git status/diff, then accumulate observed live edits.\n\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(out, "Diffplay "+version)
		return err
	}
	if *interval < 50*time.Millisecond {
		return errors.New("--interval must be at least 50ms")
	}
	if *view != "graph" && *view != "list" {
		return errors.New("--view must be graph or list")
	}
	if flags.NArg() > 1 {
		return errors.New("expected at most one directory")
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("root must be a directory")
	}
	paths := []string{root}
	grouped := false
	_, repoErr := gitrepo.Open(root)
	if errors.Is(repoErr, gitrepo.ErrNotRepository) {
		paths, err = gitrepo.Discover(root, 5)
		if err != nil {
			return fmt.Errorf("repository discovery failed: %w", err)
		}
		if len(paths) == 0 {
			return errors.New("no Git repositories found within 5 directory levels; open the application inside a Git repository")
		}
		if *snapshot {
			return errors.New("repository selection requires an interactive terminal; run --snapshot inside a Git repository")
		}
		paths, err = chooseRepositories(paths, out)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return nil
		}
		grouped = true
	} else if repoErr != nil {
		return repoErr
	}
	if !*snapshot && (!term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd()))) {
		return errors.New("an interactive terminal is required (use --snapshot for JSON output)")
	}
	if *snapshot {
		source, err := gitrepo.Open(root)
		if err != nil {
			return err
		}
		initial, scanErr := source.Scan(5)
		type item struct {
			Path      string `json:"path"`
			Directory bool   `json:"directory"`
			Size      int64  `json:"size"`
		}
		items := make([]item, 0, len(initial))
		for _, e := range initial {
			items = append(items, item{e.Path, e.IsDir, e.Size})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(items); err != nil {
			return err
		}
		return scanErr
	}
	sessions, err := loadSessions(root, paths, grouped)
	if err != nil {
		return err
	}
	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), old)
	// Alternate screen, no wrapping, hidden cursor, SGR all-motion mouse.
	if _, err = io.WriteString(out, "\x1b[?1049h\x1b[?7l\x1b[?25l\x1b[?1003h\x1b[?1006h\x1b[2J"); err != nil {
		return err
	}
	defer io.WriteString(out, "\x1b[?1006l\x1b[?1003l\x1b[?25h\x1b[?7h\x1b[0m\x1b[?1049l")
	g := newGraph()
	u := newUI(g, root)
	seedSessions(u, sessions)
	u.listMode = *view == "list"
	failures := make(map[string]error)
	for _, s := range sessions {
		failures[s.root] = s.scanErr
	}
	u.warning = sessionWarnings(sessions, failures)
	type repositoryUpdate struct {
		root   string
		update scanUpdate
	}
	updates := make(chan repositoryUpdate, len(sessions))
	done := make(chan struct{})
	defer close(done)
	for _, s := range sessions {
		go func(s repositorySession) {
			watcher := newMonitor(s.root, s.source, s.initial, s.events, s.scanErr == nil)
			ticker := time.NewTicker(*interval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					up := watcher.poll(time.Now())
					select {
					case updates <- repositoryUpdate{s.root, namespaceUpdate(s.prefix, up)}:
					case <-done:
						return
					}
				}
			}
		}(s)
	}
	input := make(chan []byte, 16)
	go func() {
		for {
			buffer := make([]byte, 256)
			n, err := os.Stdin.Read(buffer)
			if n > 0 {
				select {
				case input <- buffer[:n]:
				case <-done:
					return
				}
			}
			if err != nil {
				select {
				case input <- nil:
				case <-done:
				}
				return
			}
		}
	}()
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH)
	defer signal.Stop(signals)
	frames := time.NewTicker(50 * time.Millisecond)
	defer frames.Stop()
	for !u.quit {
		select {
		case data := <-input:
			if data == nil {
				return nil
			}
			u.input(data)
		case sig := <-signals:
			if sig != syscall.SIGWINCH {
				return nil
			}
		case result := <-updates:
			up := result.update
			failures[result.root] = up.err
			u.warning = sessionWarnings(sessions, failures)
			if up.err != nil {
				continue
			} else {
				if up.baseline {
					u.seedEvents(up.events)
				} else {
					applySnapshot(g, up.snapshot, up.events, time.Now())
					u.recordEvents(up.events)
				}
				u.applyVisibility(up.visible)
			}
		case now := <-frames.C:
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				return err
			}
			if w < 1 || h < 1 {
				continue
			}
			g.expire(now)
			u.updateGraphMetrics()
			if !u.paused && !u.listMode {
				g.step(w, max(1, h-4), 0.05)
			} else if w != g.width || max(1, h-4) != g.height {
				g.step(w, max(1, h-4), 0)
			}
			if _, err := io.WriteString(out, u.render(w, h, now)); err != nil {
				return err
			}
		}
	}
	return nil
}
