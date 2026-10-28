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

Press `Tab` to switch between list and graph; press `q` to quit.
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
