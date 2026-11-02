// Package git reads the final working tree relative to HEAD using the Git executable.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Status  string
	Path    string
	OldPath string
}

type Repository struct {
	Root string
}

func Discover(directory string) (*Repository, error) {
	output, err := run(directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("discover repository: %w", err)
	}
	return &Repository{Root: strings.TrimSuffix(string(output), "\n")}, nil
}

func run(directory string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-pager", "-C", directory}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1", "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("git %s: %s: %w", args[0], detail, err)
		}
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return output, nil
}

func (r *Repository) base() (string, error) {
	output, err := run(r.Root, "rev-parse", "--verify", "--quiet", "HEAD")
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return "", err
	}
	// Ensure the empty tree exists for diff, including in fresh SHA-1 and SHA-256 repositories.
	output, err = run(r.Root, "hash-object", "-w", "-t", "tree", "--stdin")
	return strings.TrimSpace(string(output)), err
}

func (r *Repository) Files() ([]File, error) {
	base, err := r.base()
	if err != nil {
		return nil, err
	}
	output, err := run(r.Root, "diff", "--no-ext-diff", "--no-textconv", "--name-status", "-z", "--find-renames", base, "--")
	if err != nil {
		return nil, err
	}
	var files []File
	fields := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	for i := 0; i < len(fields) && fields[i] != ""; {
		status := fields[i]
		i++
		if i >= len(fields) {
			return nil, fmt.Errorf("git diff returned an incomplete filename record")
		}
		file := File{Status: status[:1], Path: fields[i]}
		i++
		if file.Status == "R" || file.Status == "C" {
			if i >= len(fields) {
				return nil, fmt.Errorf("git diff returned an incomplete rename record")
			}
			file.OldPath, file.Path = file.Path, fields[i]
			i++
		}
		files = append(files, file)
	}
	output, err = run(r.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(string(output), "\x00") {
		if path != "" {
			files = append(files, File{Status: "?", Path: path})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Snapshot is a complete file version. Unavailable snapshots leave the diff readable
// when Git attributes, submodules, or concurrent filesystem changes prevent mapping.
type Snapshot struct {
	Path      string
	Text      string
	Available bool
}

type Diff struct {
	Text string
	Old  Snapshot
	New  Snapshot
}

func (r *Repository) Diff(file File) (Diff, error) {
	if file.Status == "?" {
		content, err := r.worktreeContent(file.Path)
		if err != nil {
			return Diff{}, err
		}
		return Diff{Text: untrackedDiff(file.Path, content), New: Snapshot{Path: file.Path, Text: string(content), Available: true}}, nil
	}
	base, err := r.base()
	if err != nil {
		return Diff{}, err
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--submodule=short", base, "--", file.Path}
	if file.OldPath != "" {
		args = append(args, file.OldPath)
	}
	output, err := run(r.Root, args...)
	if err != nil {
		return Diff{}, err
	}
	diff := Diff{Text: string(output)}
	oldPath := file.OldPath
	if oldPath == "" {
		oldPath = file.Path
	}
	if file.Status != "A" {
		content, readErr := run(r.Root, "cat-file", "blob", base+":"+oldPath)
		diff.Old = Snapshot{Path: oldPath, Text: string(content), Available: readErr == nil}
	}
	if file.Status != "D" {
		content, readErr := r.worktreeContent(file.Path)
		diff.New = Snapshot{Path: file.Path, Text: string(content), Available: readErr == nil}
	}
	return diff, nil
}

func (r *Repository) worktreeContent(path string) ([]byte, error) {
	fullPath := filepath.Join(r.Root, path)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var content []byte
	if info.Mode()&os.ModeSymlink != 0 {
		var target string
		target, err = os.Readlink(fullPath)
		content = []byte(target)
	} else if info.Mode().IsRegular() {
		content, err = os.ReadFile(fullPath)
	} else {
		return nil, fmt.Errorf("cannot display %q: not a regular file or symlink", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return content, nil
}

func untrackedDiff(path string, content []byte) string {
	if bytes.ContainsRune(content, '\x00') {
		return fmt.Sprintf("Binary file %q (untracked)\n", path)
	}
	var diff strings.Builder
	fmt.Fprintf(&diff, "--- /dev/null\n+++ %q\n", "b/"+path)
	if len(content) == 0 {
		diff.WriteString("(empty untracked file)\n")
		return diff.String()
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	fmt.Fprintf(&diff, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		diff.WriteString("+" + line + "\n")
	}
	if content[len(content)-1] != '\n' {
		diff.WriteString("\\ No newline at end of file\n")
	}
	return diff.String()
}
