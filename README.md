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
diffplay                        # Watch the current directory in list view.
diffplay --view graph           # Start in graph view.
diffplay /path/to/repository    # Watch a specific directory.
```

Press `Tab` to cycle through **Clusters → List → Spiral → Hex → Clusters**; press `q` to quit. The list is the existing change list; the other three views are experimental cluster visualizations for comparison, not a final selection.

All cluster variants preserve the same folder hierarchy and file identities. What changes is the spatial packing algorithm and the edit symbol:

- **Clusters**: size-ordered concentric contact rings. Solid disks use outer/core areas for dominant/minority edits, with size proportional to cumulative edited lines within each repository.
- **Spiral**: golden-angle candidate placement with spatial-hash collision rejection, at every hierarchy level. Pie-sector disks show additions and removals separately; blue marks additions to new files.
- **Hex**: axial hexagonal lattice packing at every hierarchy level. Equal-capacity file glyphs keep small changes visible beside large ones. Braille dots in diamond-shaped glyphs show additions on the left and removals on the right. One dot represents one line when space permits; larger counts are aggregated with the exact per-file unit shown on hover (rounded up).

When a glyph is too small for detail, `+`, `−`, or `±` preserves the edit direction; `?` means unavailable counts and `·` means no counted line edits. Hover or `n/p` reveals the file, exact cumulative counts, and latest actual diff. Wheel/`+`/`-` zoom, `r` resets zoom, `j/k` scroll the diff, and dragging stays within the file's cluster. Switching algorithms resets manual drag offsets.

Layouts are deterministic and stationary at rest: no force simulation or ambient motion. Old deleted markers are retired only alongside a subsequent file change. Short bursts follow live edits only; startup changes stay quiet.
Git ignore rules are respected. Open a Git repository, or a directory containing repositories to select from.
Repository discovery inspects only the selected directory and its immediate children; it never recurses into grandchildren.
Hidden children and reserved system/dependency directories (`Library`, `System`, `Applications`, `Windows`, `node_modules`, `vendor`, `__pycache__`, and `venv`) are skipped, as are inaccessible children.
An explicitly selected repository remains valid even if its directory is hidden or reserved. Symlink directories and symlink `.git` markers are not followed during discovery; linked Git worktrees are supported.

## Development

```sh
make test
make build
./bin/diffplay
```

## License

[MIT](LICENSE)
