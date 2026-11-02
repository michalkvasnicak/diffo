# diffo

A read-only Git diff TUI for small terminals, built with Bubble Tea and Lip Gloss.

## Getting started

Requires Go 1.27 or newer, Git on `PATH`, and an interactive terminal.

```sh
go run ./cmd/diffo
```

Run from a Git working tree or any of its subdirectories. To build or install:

```sh
go build -o bin/diffo ./cmd/diffo
go install ./cmd/diffo
```

The full-screen file picker shows **`git diff HEAD` changes**, plus non-ignored untracked files. Staged and unstaged changes are not separate groups: staged edits canceled by working-tree edits do not appear. Repositories without commits compare against an empty tree; diffo ensures that harmless tree object exists in Git's object database without modifying the index or working files.

Select a file to read its unified diff. Deleted files show removed content, untracked text files show additions, and binary changes show a notice rather than binary data. Renames detected by Git appear with their old path. Status letters include `M` modified, `A` added, `D` deleted, `R` renamed, `T` type changed, and `?` untracked.

## Controls

| Key | Action |
| --- | --- |
| `j` / `k`, arrow keys | Select files or scroll diff |
| `Enter`, `l`, right arrow | Open selected file |
| `h`, `Esc`, left arrow | Return to file picker |
| `PageDown` / `PageUp`, `Ctrl+f` / `Ctrl+b` | Move one page |
| `g` / `G`, Home / End | Go to start / end |
| `r` | Refresh file list (also returns from diff) |
| `q`, `Ctrl+c` | Quit |

Mouse wheel scrolls; clicking a file opens it when supported by the terminal. Headers and file rows truncate to terminal width; diffs wrap at terminal-cell boundaries and scroll vertically. Addition/deletion colors supplement the `+` / `-` markers. Git queries run asynchronously with loading, empty, and error states.

This is a snapshot browser, not a live watcher: refresh after external changes. Git's index still determines which files are tracked: after `git rm --cached`, a remaining or recreated file can appear twice, as a deletion and as an untracked file, even if its contents match HEAD. Open the `?` row to read its current contents. Diffs are loaded when opened, so a file changed since the list was loaded may show no diff or an error; refresh to reconcile the list. Files and diffs are held in memory, so very large changes may be slow. Terminal controls in content are replaced for safe display and tabs expand to four spaces. There is no syntax highlighting, editing, staging, or branch comparison. Git's own attributes and diff configuration still influence how it represents files; external diff tools and text-conversion commands are disabled.

## Structure

- `cmd/diffo/`: executable entry point and process-level error handling.
- `internal/app/`: terminal setup and dependency wiring.
- `internal/git/`: repository discovery, final worktree change queries, and diff content.
- `internal/tui/`: Bubble Tea state, input, and responsive rendering; no Git I/O during rendering.

The module is named `diffo` until a canonical repository URL is chosen.

## Automatic checks

`.pi/settings.json` loads `@michalkvasnicak/pi-hooks`, following the setup in `../piup`. Start Pi from the repository root and grant project trust. Reload Pi after initially adding the package.

Before the agent finishes, `.pi/pi-hooks.json` runs these checks in order:

1. Format Go source with `gofmt -w`.
2. Run Go's built-in static analysis with `go vet ./...`.
3. Run tests with `go test -race ./...` (requires a C compiler for the race detector).

Checks rerun when their inputs change, and failures return to the agent for correction. No tests are included yet.
