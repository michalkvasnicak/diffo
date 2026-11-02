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

The full-screen file tree shows **`git diff HEAD` changes**, plus non-ignored untracked files. Directories appear before files and start expanded. Collapse or expand them with `h`/`l` or the left/right arrows; `Enter` or a mouse click toggles a directory or opens a file. Each directory shows its descendant changed-file count and aggregate status badges. Files show colored status letters, and renamed files include their old path. Expansion state is retained on refresh for directories that remain in the tree. Staged and unstaged changes are not separate groups: staged edits canceled by working-tree edits do not appear. Repositories without commits compare against an empty tree; diffo ensures that harmless tree object exists in Git's object database without modifying the index or working files.

Select a file to read its unified diff, then press `v` to toggle the **annotated full file**. This view includes every current working-tree line, plus standalone deleted lines inline at their original boundaries. The gutter shows current-file line numbers; standalone deleted lines use their old line number prefixed with `-`. There are no change prefixes in the source text itself. Unchanged characters have no background; additions have a green background. Replacements show only the new text: inserted graphemes are green, while graphemes replacing deleted text are orange, not the entire line or unchanged token portions (`count = 10` → `count = 20` highlights only `2`; `foobar` → `foo` shows just `foo`, uncolored). Deletion-only blocks retain the old text with a red background. Entire added/untracked files are green; entire deleted files show HEAD in red (using the old path when present). The header labels the annotated mode and source. Empty files, binary files, and unavailable snapshots show readable notices; an unavailable current snapshot never silently falls back to HEAD. Deleted diffs show removed content, untracked text diffs show additions, and binary changes show a notice rather than binary data. Renames detected by Git appear with their old path. Status letters include `M` modified, `A` added, `D` deleted, `R` renamed, `T` type changed, and `?` untracked.

## Controls

| Key | Action |
| --- | --- |
| `j` / `k`, up/down arrows | Select tree rows or scroll content |
| `v` | Toggle unified diff / annotated full file |
| `n` / `p` | Next / previous hunk in either view (no wraparound) |
| `N` / `P`, Tab / Shift+Tab | Next / previous changed file in either view (no wraparound) |
| `Enter` | Toggle selected directory or open selected file |
| `l`, right arrow | Expand directory, enter expanded directory, or open file |
| `h`, left arrow | Collapse directory or select parent; return from content to tree |
| `Esc` | Return from content to tree |
| `PageDown` / `PageUp`, `Ctrl+f` / `Ctrl+b` | Move one page |
| `g` / `G`, Home / End | Go to start / end |
| `r` | Refresh file list (also returns from diff) |
| `q`, `Ctrl+c` | Quit |

In the diff, hunk jumps select the next / previous header relative to the viewport top. If navigation has no remembered target and a hunk target is already at the top, the first press selects it. In full-file mode, they target each hunk's first displayed edit line, including standalone deletions. Targets follow wrapped terminal rows in both views; unavailable annotations have no full-file hunk targets. Near the end, scrolling stops at the bottom but subsequent jumps advance from the last targeted hunk, so closely spaced final hunks remain navigable. Cyan highlighting on the line numbers and their right-hand `│` separator marks only added, deleted, or replaced lines in the selected hunk, including their wrapped continuation rows, without overriding source change backgrounds. Unchanged context and hunk headers remain unmarked. Scrolling manually, resizing, or toggling view mode resets the navigation anchor but retains the selected hunk. Subsequent jumps use the viewport position; opening another file clears the selection. No hunks means no jump; reaching either hunk or file boundary does nothing. Toggling the same file preserves separate diff and full-file scroll positions, clamped to the available rows when resizing. Switching files retains the view mode but resets both scroll positions, and also works while loading or after an error. Returning to the picker keeps the current file selected and visible; opening from the picker retains the last selected view mode. The mode is remembered only for the current process, including across refreshes; a new CLI run starts in diff mode. Toggling during loading records the desired mode.

Mouse wheel scrolls; clicking a file opens it and clicking a directory toggles it when supported by the terminal. Headers and file rows truncate to terminal width; both content views wrap at terminal-cell boundaries and scroll vertically. Unified diffs have separate old/new line-number columns; full-file mode has one number column. Number columns are muted and sized to their displayed coordinates; continuation rows leave numbers blank. Content wraps within the space left after the gutters. Very narrow terminals hide the line-number gutter, including its selection highlight, to leave room for source text. Chroma syntax foregrounds are preserved over subtle addition/deletion/replacement backgrounds, including within a syntax token. Unified diff keeps its existing whole-line backgrounds and `+` / `-` markers. Colors follow terminal capabilities and respect `NO_COLOR`. Git queries, syntax highlighting, and annotation alignment run asynchronously with loading, empty, and error states. Both views reuse the same loaded snapshots and syntax tokens; toggling does not requery Git or retokenize, and resizing only rewraps cached highlighted lines.

Annotation alignment treats a contiguous removed/added block as a replacement. Exact matching lines anchor the alignment; between anchors, old and new lines pair in order. Each paired line is compared by grapheme, preserving unchanged portions without backgrounds. An uninterrupted edit group with both removed and inserted text is orange; insertion-only groups are green, including appended comments on existing lines. Extra new lines are green; all old text in a replacement disappears, including extra old lines and deleted character portions. Only blocks with no added lines remain red. Alignment is deterministic, not a semantic understanding of moved or rewritten code: ambiguous/reordered lines can pair differently than a human would choose. Newline-at-EOF changes and empty lines have no characters to color. Alignment operates on edit-block line IDs and paired-line graphemes, not an entire-file character comparison; pathological large replacement blocks or very long rewritten lines can still be expensive.

This is a snapshot browser, not a live watcher: refresh after external changes. Git's index still determines which files are tracked: after `git rm --cached`, a remaining or recreated file can appear twice, as a deletion and as an untracked file, even if its contents match HEAD. Open the `?` row to read its current contents. Diffs are loaded when opened, so a file changed since the list was loaded may show no diff or an error; refresh to reconcile the list. Files and diffs are held in memory, so very large changes may be slow. Terminal controls in content are replaced for safe display and tabs expand to four spaces. There is no editing, staging, or branch comparison. Git's own attributes and diff configuration still influence how it represents files; external diff tools and text-conversion commands are disabled.

## Syntax highlighting

Syntax is selected by filename using Chroma's pure-Go lexers and a light/dark terminal palette. Complete HEAD and working-tree snapshots are tokenized before mapping to hunk line numbers, preserving multiline comments and strings even when they begin outside a hunk. Deleted lines use HEAD and its old filename; added and context lines use the current file. Added, untracked, deleted, and renamed files are supported. Diff headers and hunk metadata remain separate from source syntax.

Unknown languages remain plain text. Binary notices, symlink targets, and Git's custom diff representations remain readable. If a hunk does not exactly match the snapshots (for example due to attributes, line-ending conversion, or concurrent edits), its syntax highlighting is skipped rather than styling the wrong source. Multi-patch responses also fall back to plain diff styling. Annotated mode additionally verifies all unchanged gaps against both snapshots. When the patch cannot be mapped completely, it explicitly reports unavailable annotations and shows the selected snapshot without change backgrounds rather than inventing annotations. Very large files require full-file tokenization and may take time and memory.

## Structure

- `cmd/diffo/`: executable entry point and process-level error handling.
- `internal/app/`: terminal setup and dependency wiring.
- `internal/git/`: repository discovery, final worktree change queries, and diff content.
- `internal/diff/`: plain unified-diff parsing, snapshot matching, complete-file annotation mapping, and line/grapheme alignment with safe mapping fallbacks.
- `internal/highlight/`: full-source Chroma tokenization and syntax foregrounds with a terminal-aware theme.
- `internal/terminaltext/`: terminal-control sanitization, single-line escaping, and tab expansion.
- `internal/tui/`: async composition of parsed diffs and highlighted snapshots into cached presentation, plus Bubble Tea state, input, and responsive rendering; no Git I/O or tokenization during rendering. `model.go` coordinates navigation and async loading; `browser.go` owns the file-tree screen; `viewer.go` owns the diff/annotated-file screen and scrolling; `tree.go` is the tree component; `layout.go` shares header/body/footer layout and styles; `gutter.go` wraps content with line numbers and selected-hunk indicators. Screen models handle their own input and return navigation requests to the root.

The module is named `diffo` until a canonical repository URL is chosen.

## Automatic checks

`.pi/settings.json` loads `@michalkvasnicak/pi-hooks`, following the setup in `../piup`. Start Pi from the repository root and grant project trust. Reload Pi after initially adding the package.

Before the agent finishes, `.pi/pi-hooks.json` runs these checks in order:

1. Format Go source with `gofmt -w`.
2. Run Go's built-in static analysis with `go vet ./...`.
3. Run tests with `go test -race ./...` (requires a C compiler for the race detector).

Checks rerun when their inputs change, and failures return to the agent for correction. No tests are included yet.
