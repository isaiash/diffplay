# Diffplay

Display the differences in your files made by your agents.

Watch live file changes in your terminal with a sorted change list and an interactive graph.

## Install (macOS and Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/isaiash/diffplay/main/quickinstall.sh | bash
```

The installer detects your OS, installs missing dependencies (may require sudo or Homebrew), and downloads a temporary Go toolchain when needed.
The binary is installed to `~/.local/bin` by default; make sure that directory is on your `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

### From source

With Go 1.24+ and Git installed, run this from a checkout:

```sh
./install.sh
```

## Usage

```sh
diffplay                        # Combined view: list/diff above, graph below.
diffplay --view list            # Start with the full-screen change list.
diffplay --view graph           # Start in graph view.
diffplay /path/to/repository    # Watch a specific directory.
```

The default **Combined** view starts with a horizontal split near half-height: the change list and its right-hand diff preview occupy the upper pane, and the Hex graph occupies the lower pane. Drag either divider with the left mouse button to resize the panes; the split proportions are retained when the terminal is resized. Hover over a divider for its `↕` or `↔` handle. Press `Tab` to cycle **Combined → List → Hex → Combined**, or `q` to quit. Use `--view combined`, `--view list`, or `--view graph` to choose the initial view explicitly.

Press `m` to select and copy text using your terminal's mouse selection and copy shortcut (commonly `Ctrl+Shift+C` on Linux or `Cmd+C` on macOS). The screen stays still while selecting; monitoring continues. Press `m` again to resume interaction and show the latest updates. Diff tabs are expanded to eight-column stops, CRLF is displayed normally, and Unicode text is preserved; unsafe terminal control characters are not executed.

The first row shows the repository name, cumulative additions/removals, and `(added - removed) / current repository lines` as a two-decimal percentage labeled **new lines**. The current line count is right-aligned as **total lines**. The background bar represents the full repository: positive net additions apportioned between new files (blue) and existing files (green), removals (red), then the remaining lines on the terminal's default background. Negative net growth remains a negative percentage; bar segments cannot have negative widths. Repeated edits can exceed the corpus, so colored segments then fill the bar proportionally with no neutral remainder; this is activity, not distinct-line coverage.

The full-depth denominator is updated in the background. NUL-containing binary files and symlinks are excluded; invalid UTF-8, files over 64 MiB, or counting errors make the percentage unavailable (`?%`) rather than approximate. Monitoring the changes themselves still uses depth five. The combined view has a single help footer at the bottom. New, modified, and removed lines use fixed blue, green, and red RGB colors so a terminal theme cannot change their meaning. This requires a terminal with 24-bit color support. Structural lines and help use the terminal's normal foreground for contrast, while quiet context uses a dimmed default foreground; Diffplay does not change the terminal theme.

Selection is synchronized in both directions. Navigating the list paints the corresponding file's existing dots white without changing its shape, and brightens every ancestor folder frame. Hovering/selecting a graph file reveals and scrolls to that file in the list, opening its ancestor folders. Selecting a gray context file temporarily shows a zero-count context row without adding activity or changing totals. In Combined, the diff stays in the upper-right preview rather than covering the graph. List histograms use the largest individual file change within each repository as a stable scale independent of scrolling; collapsed folder aggregates can fill the bar.

Live file changes open their ancestor folders, reveal the file and show its current diff against `HEAD`. A white number before the added-line count records observed file-change bursts; zero is blank. Counts persist per repository in the user's configuration directory and reset for each file when its changes are committed or reverted. Changes between polling scans can be missed. If storage or the `HEAD` preview is unavailable, monitoring continues and the header displays a warning.

Hex packs files on a deterministic hexagonal lattice inside compact rectangular folder frames. Each frame carries only its local folder name followed by `/`; subfolder frames nest inside their parents, and the redundant `./` root frame is omitted. Entire folder branches without changed files are hidden recursively and appear as soon as a file changes. Unchanged files directly inside active folders and their ancestors remain as context; quiet child folders do not. Sibling frames sit close together rather than reserving large circular envelopes. Each file is a circular cell made of Braille dots:

- **Body:** the majority edit color—green for additions to existing files, light blue for additions to new files, or red for removals.
- **Nucleus:** the minority edit color, centered inside the body. Its share follows the minority fraction of cumulative edited lines; a +30/-10 file has a green body and a red nucleus representing roughly one quarter of its dots. A single edit direction needs no nucleus. Deleted files are red.
- **Context:** unmodified, non-ignored files appear as faint gray dotted cells whose area is proportional to the file's line count, normalized within each repository. Empty files keep a minimum visible marker; unavailable line counts use a small fallback rather than an estimated count. They are inspectable in the graph but do not enter the changed-only list or its totals. Changed files retain equal available capacity and use dot density for cumulative edits.

For changed files, one dot represents one edited line when space permits; larger counts are aggregated, with the exact per-file unit shown on hover (rounded up). Color proportions are approximate at terminal-cell resolution because each Braille character has only one foreground color; zoom in to resolve small nuclei. Unknown counts remain marked in the inspection panel. Gray context uses exact current UTF-8 text line counts up to 8 MiB, including files whose diff-content capture is truncated; larger files, binary files, and symlinks use the unavailable-count fallback. Very small files retain a minimum visible size.

Hover or `n/p` reveals the file, exact cumulative counts, and latest actual diff. The mouse wheel zooms toward the pointer; `+`/`-` also zoom. Left-button dragging pans the entire graph, whether started on a file or empty space. `r` resets zoom and position, and `j/k` scroll the diff.

Layouts adapt to the viewport and repack when files appear or disappear. They are deterministic and stationary at rest: no force simulation or ambient motion. A live edit briefly flashes its file white and emits a fast fading ripple; startup changes stay quiet. When an untracked file is deleted, its activity and graph cell disappear instead of producing a removal diff. Files tracked by Git still show deletions; old deleted markers are retired only alongside a subsequent file change. Changes entirely between polling scans can be missed.
Git ignore rules are respected. Open a Git repository, or a directory containing repositories to select from.
Repository discovery inspects only the selected directory and its immediate children; it never recurses into grandchildren.
Hidden children and reserved system/dependency directories (`Library`, `System`, `Applications`, `Windows`, `node_modules`, `vendor`, `__pycache__`, and `venv`) are skipped, as are inaccessible children.
An explicitly selected repository remains valid even if its directory is hidden or reserved. Symlink directories and symlink `.git` markers are not followed during discovery; linked Git worktrees are supported.

## Development

```sh
make test
make build
make smoke
./bin/diffplay
```

## License

[MIT](LICENSE)
