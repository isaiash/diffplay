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

Use your terminal's native mouse selection override (usually `Shift` + drag) to select diff text, then use your own configured copy shortcut or universal copier. No `m` mode or application-specific clipboard command is needed. Diffplay avoids identical idle redraws so they do not clear the selection; live updates still redraw and may affect it. The exact selection modifier and rectangular-selection gesture depend on the terminal. Diff tabs are expanded to eight-column stops, CRLF is displayed normally, and Unicode text is preserved; unsafe terminal control characters are not executed.

The first row shows cumulative additions/removals and `(added - removed) / current repository lines` as a two-decimal percentage labeled **new lines**, without the repository name. The current line count is right-aligned as **total lines**. The background bar represents only the cumulative diffs: additions in new files (blue), additions in existing files (green), and removals (red), apportioned across the entire width. Unchanged repository lines do not occupy a segment. The bar is independent of the repository line count; when edit counts are unknown or no edits exist, it remains neutral. Negative net growth remains a negative percentage.

The list's next row labels **HITS**, **ADDED**, the untitled histogram, **REMOVED**, **%**, and the repository name over the file paths. The percentage column is `(added + removed) / current file lines × 100`, displayed in four cells without a repeated `%`: for example, `100`, `10.4`, or `0.23`. Cumulative edits can exceed 100%; values too large for three digits show `>999`, and tiny nonzero values below the display precision show `<.01`. Unavailable counts, deleted files, or a nonzero change count divided by zero display `?`. Directory percentages are left blank. Panes narrower than 40 columns use compact labels and omit the percentage.

The full-depth denominator is updated in the background. NUL-containing binary files and symlinks are excluded; invalid UTF-8, files over 64 MiB, or counting errors make the percentage unavailable (`?%`) rather than approximate. Git snapshots, startup changes, and live monitoring include allowed files at every depth, including deeply nested source packages; ignored paths and Git metadata remain excluded. The combined view has a single help footer at the bottom. New, modified, and removed lines use fixed blue, green, and red RGB colors so a terminal theme cannot change their meaning. This requires a terminal with 24-bit color support. Structural lines and help use the terminal's normal foreground for contrast, while quiet context uses a dimmed default foreground; Diffplay does not change the terminal theme.

Selection is synchronized in both directions. Navigating the list paints the corresponding file's existing dots white without changing its shape, and brightens every ancestor folder frame. Hovering/selecting a graph file reveals and scrolls to that file in the list, opening its ancestor folders. In the full-screen graph, selecting a gray context file temporarily shows a zero-count context row without adding activity or changing totals. In Combined, only changed files appear in the graph and the diff stays in the upper-right preview rather than covering the graph. List histograms use the largest individual file change within each repository as a stable scale independent of scrolling; collapsed folder aggregates can fill the bar.

Live file changes open their ancestor folders, reveal the file and show its current diff against `HEAD`. Each hit produces a bright event-colored row background that fades rapidly over one second, returning to the normal or selected background. A white number before the added-line count records observed file-change bursts; zero is blank. Counts persist per repository in the user's configuration directory and reset for each file when its changes are committed or reverted. Changes between polling scans can be missed. If storage or the `HEAD` preview is unavailable, monitoring continues and the header displays a warning.

The fade uses read-only OSC color queries to interpolate toward the terminal's actual background and selection palette color, without waiting for replies or changing the theme. Terminals that do not answer use dark-theme estimates during the fade and restore their original background after one second.

Hex packs files on a deterministic hexagonal lattice inside compact rectangular folder frames. Each frame carries only its local folder name followed by `/`; subfolder frames nest inside their parents, and the redundant `./` root frame is omitted. Entire folder branches without changed files are hidden recursively and appear as soon as a file changes. Sibling frames sit close together rather than reserving large circular envelopes. Each file is a circular cell made of Braille dots.

In the **Combined** split view, unchanged files are hidden. Each changed file is a filled disk whose area follows its cumulative added-plus-removed line count, normalized within each repository. Its layers represent only additions in new files (blue), additions in existing files (green), and removals (red). The largest quantity is the outermost layer, with successively smaller quantities nested concentrically toward the center. No gray unchanged-line layer or total-file-size information is shown. All interior dots are filled rather than using sparse edit density; zero or unavailable edit counts retain a minimum marker.

The **full-screen graph** retains unchanged files directly inside active folders and their ancestors as context; quiet child folders stay hidden. Its original edit-density representation is unchanged:

- **Body:** the majority edit color—green for additions to existing files, light blue for additions to new files, or red for removals.
- **Nucleus:** the minority edit color, centered inside the body. Its share follows the minority fraction of cumulative edited lines; a +30/-10 file has a green body and a red nucleus representing roughly one quarter of its dots. A single edit direction needs no nucleus. Deleted files are red.
- **Context:** unmodified, non-ignored files appear as faint gray dotted cells whose area is proportional to the file's line count, normalized within each repository. Empty files keep a minimum visible marker; unavailable line counts use a small fallback rather than an estimated count. They are inspectable in the graph but do not enter the changed-only list or its totals. Changed files retain equal available capacity and use dot density for cumulative edits.

In the full-screen graph, one dot represents one edited line when space permits; larger counts are aggregated, with the exact per-file unit shown on hover (rounded up). Color proportions in both views are approximate at terminal-cell resolution because each Braille character has only one foreground color; zoom in to resolve small nuclei. Unknown counts remain marked in the inspection panel. File sizes use exact current UTF-8 text line counts up to 8 MiB, including files whose diff-content capture is truncated; larger files, binary files, and symlinks use the unavailable-count fallback. Very small files retain a minimum visible size.

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
