package app

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


func applySnapshot(g *graph, s watch.Snapshot, events []changeEvent, now time.Time) {
	// Scanned files provide quiet graph-only context. Missing snapshot paths are
	// not deletions: only real events and ignore visibility retire existing nodes.
	// In a workspace each snapshot belongs to just one repository.
	for id, entry := range s {
		if id != "." {
			g.upsert(id, path.Dir(id), entry.IsDir)
			n := g.nodes[id]
			n.lines, n.linesKnown = entry.Lines, entry.LinesKnown
		}
	}
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
	// Retire old tombstones only alongside a file change, never on idle polls.
	for _, e := range events {
		if !e.IsDir {
			g.expire(now)
			break
		}
	}
}
// Run executes the command without terminating the caller's process.
func Run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("diffplay", flag.ContinueOnError)
	flags.SetOutput(out)
	interval := flags.Duration("interval", 400*time.Millisecond, "filesystem polling interval (minimum 50ms)")
	snapshot := flags.Bool("snapshot", false, "print a JSON file listing and exit; no terminal required")
	showVersion := flags.Bool("version", false, "print version and exit")
	view := flags.String("view", "combined", "initial view: combined, list or graph (Tab cycles views)")
	flags.Usage = func() {
		fmt.Fprintln(out, "Diffplay\n\nUsage: diffplay [options] [directory]\n\nLive repository activity inside your terminal: Hex lattice and sorted change tree.\nDefaults to the current directory; Git-ignored paths are excluded; depth <= 5.\nOutside Git, inspect only the selected directory and its immediate children; never recurse.\nHidden and reserved system/dependency children are skipped.\nStartup Git changes are loaded quietly; only subsequent live changes flash.\n\nTab: cycle combined → list → Hex graph. Combined: change list + diff above, Hex below; drag the divider to resize.\nMouse focuses each pane; keyboard starts in list; J/K scroll diff; n/p select files.\nList: arrows navigate, right expands, left collapses,\nclick toggles folders, hover selects, wheel scrolls, a/c expands/collapses all.\nGraph: dotted cells with majority body/minority nucleus; faint gray unchanged files.\nHover diff, drag to pan, wheel zoom at pointer, +/- zoom, r resets, n/p selects files.\nq: quit. Counts start from Git status/diff, then accumulate observed live edits.\n\nOptions:")
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
	if *view != "graph" && *view != "list" && *view != "combined" {
		return errors.New("--view must be combined, list or graph")
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
		paths, err = gitrepo.Discover(root, 1)
		if err != nil {
			return fmt.Errorf("repository discovery failed: %w", err)
		}
		if len(paths) == 0 {
			return errors.New("no Git repositories found in the selected directory or its immediate children; open the application inside a Git repository")
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
	if _, err = io.WriteString(out, "\x1b[?1049h\x1b[?7l\x1b[?25l\x1b[?1003h\x1b[?1006h\x1b[2J"+terminalColorQueries); err != nil {
		return err
	}
	defer io.WriteString(out, "\x1b[?1006l\x1b[?1003l\x1b[?25h\x1b[?7h\x1b[0m\x1b[?1049l")
	g := newGraph()
	u := newUI(g, root)
	seedSessions(u, sessions)
	u.listMode = *view == "list"
	u.combined = *view == "combined"
	failures := make(map[string]error)
	for _, s := range sessions {
		failures[s.root] = errors.Join(s.scanErr, s.activityErr)
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
			watcher := s.newMonitor()
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
			failures[result.root] = errors.Join(up.err, up.warning)
			u.warning = sessionWarnings(sessions, failures)
			u.applyScanUpdate(up)
		case now := <-frames.C:
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				return err
			}
			if w < 1 || h < 1 {
				continue
			}

			if _, err := io.WriteString(out, u.draw(w, h, now)); err != nil {
				return err
			}
		}
	}
	return nil
}
