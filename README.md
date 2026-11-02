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

Select a file to read its unified diff, then press `v` to toggle full-file content. Full-file mode shows the complete current working-tree snapshot for modified, added, renamed, and untracked files; deleted files show the original HEAD snapshot (using the old path when present). The header labels the mode and source. Empty files, binary files, and unavailable snapshots show readable notices; an unavailable current snapshot never silently falls back to HEAD. Deleted diffs show removed content, untracked text diffs show additions, and binary changes show a notice rather than binary data. Renames detected by Git appear with their old path. Status letters include `M` modified, `A` added, `D` deleted, `R` renamed, `T` type changed, and `?` untracked.

## Controls

| Key | Action |
| --- | --- |
| `j` / `k`, arrow keys | Select files or scroll content |
| `v` | Toggle unified diff / full file |
| `n` / `p` | Next / previous hunk in diff (no wraparound) |
| `N` / `P`, Tab / Shift+Tab | Next / previous changed file in either view (no wraparound) |
| `Enter`, `l`, right arrow | Open selected file |
| `h`, `Esc`, left arrow | Return to file picker |
| `PageDown` / `PageUp`, `Ctrl+f` / `Ctrl+b` | Move one page |
| `g` / `G`, Home / End | Go to start / end |
| `r` | Refresh file list (also returns from diff) |
| `q`, `Ctrl+c` | Quit |

In the diff, hunk jumps select the first header strictly below / above the viewport top. Near the end, scrolling stops at the bottom but subsequent jumps advance from the last targeted hunk, so closely spaced final hunks remain navigable. Scrolling manually or resizing resets that target. No hunks means no jump; reaching either hunk or file boundary does nothing. Hunk jumps do nothing in full-file mode. Toggling the same file preserves separate diff and full-file scroll positions, clamped to the available rows when resizing. Switching files retains the view mode but resets both scroll positions, and also works while loading or after an error. Returning to the picker keeps the current file selected and visible; opening from the picker always starts in diff mode. Toggling during loading records the desired mode.

Mouse wheel scrolls; clicking a file opens it when supported by the terminal. Headers and file rows truncate to terminal width; both content views wrap at terminal-cell boundaries and scroll vertically. Chroma syntax foregrounds are preserved over subtle addition/deletion backgrounds, alongside the `+` / `-` markers. Colors follow terminal capabilities and respect `NO_COLOR`. Git queries and syntax highlighting run asynchronously with loading, empty, and error states. Full-file content uses syntax foregrounds without diff markers or addition/deletion backgrounds. Both views are prepared from the same loaded snapshots; toggling does not requery Git or retokenize, and resizing rewraps the cached highlighted lines.

This is a snapshot browser, not a live watcher: refresh after external changes. Git's index still determines which files are tracked: after `git rm --cached`, a remaining or recreated file can appear twice, as a deletion and as an untracked file, even if its contents match HEAD. Open the `?` row to read its current contents. Diffs are loaded when opened, so a file changed since the list was loaded may show no diff or an error; refresh to reconcile the list. Files and diffs are held in memory, so very large changes may be slow. Terminal controls in content are replaced for safe display and tabs expand to four spaces. There is no editing, staging, or branch comparison. Git's own attributes and diff configuration still influence how it represents files; external diff tools and text-conversion commands are disabled.

## Syntax highlighting

Syntax is selected by filename using Chroma's pure-Go lexers and a light/dark terminal palette. Complete HEAD and working-tree snapshots are tokenized before mapping to hunk line numbers, preserving multiline comments and strings even when they begin outside a hunk. Deleted lines use HEAD and its old filename; added and context lines use the current file. Added, untracked, deleted, and renamed files are supported. Diff headers and hunk metadata remain separate from source syntax.

Unknown languages remain plain text. Binary notices, symlink targets, and Git's custom diff representations remain readable. If a hunk does not exactly match the snapshots (for example due to attributes, line-ending conversion, or concurrent edits), its syntax highlighting is skipped rather than styling the wrong source. Multi-patch responses also fall back to plain diff styling. Very large files require full-file tokenization and may take time and memory.

## Structure

- `cmd/diffo/`: executable entry point and process-level error handling.
- `internal/app/`: terminal setup and dependency wiring.
- `internal/git/`: repository discovery, final worktree change queries, and diff content.
- `internal/highlight/`: full-snapshot Chroma tokenization, safe display text, and unified-diff syntax mapping.
- `internal/tui/`: Bubble Tea state, input, and responsive rendering; no Git I/O or tokenization during rendering.

The module is named `diffo` until a canonical repository URL is chosen.

## Automatic checks

`.pi/settings.json` loads `@michalkvasnicak/pi-hooks`, following the setup in `../piup`. Start Pi from the repository root and grant project trust. Reload Pi after initially adding the package.

Before the agent finishes, `.pi/pi-hooks.json` runs these checks in order:

1. Format Go source with `gofmt -w`.
2. Run Go's built-in static analysis with `go vet ./...`.
3. Run tests with `go test -race ./...` (requires a C compiler for the race detector).

Checks rerun when their inputs change, and failures return to the agent for correction. No tests are included yet.
