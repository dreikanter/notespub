// Package deploy manages a local working copy of the deploy target git
// repository, where the built site is committed and pushed.
package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CacheRoot returns the root directory used for deploy working copies.
func CacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "npub"), nil
}

// WorkDir returns the working directory for a given deploy repo URL.
func WorkDir(repoURL string) (string, error) {
	root, err := CacheRoot()
	if err != nil {
		return "", err
	}
	slug := RepoSlug(repoURL)
	if slug == "" {
		return "", fmt.Errorf("cannot derive directory name from %q", repoURL)
	}
	return filepath.Join(root, slug), nil
}

// RepoSlug derives a directory name from a git repository URL. It strips a
// trailing ".git" and returns the final path component, suitable as a local
// directory name.
func RepoSlug(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// Options controls Prepare, Commit, and Push.
type Options struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (o Options) writers() (io.Writer, io.Writer) {
	out := o.Stdout
	if out == nil {
		out = os.Stdout
	}
	errw := o.Stderr
	if errw == nil {
		errw = os.Stderr
	}
	return out, errw
}

// Prepare ensures workDir contains an up-to-date checkout of repoURL. If
// workDir does not exist, repoURL is cloned into it. Otherwise the remote
// is fetched and the working copy is hard-reset to the remote default
// branch, discarding local commits and untracked files so the next build
// starts from a clean state.
func Prepare(repoURL, workDir string, opt Options) error {
	stdout, stderr := opt.writers()
	info, err := os.Stat(workDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
			return fmt.Errorf("creating cache parent: %w", err)
		}
		return runGit(stdout, stderr, "", "clone", repoURL, workDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", workDir)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err != nil {
		return fmt.Errorf("not a git working copy: %s", workDir)
	}
	if err := runGit(stdout, stderr, workDir, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	branch, err := DefaultBranch(workDir)
	if err != nil {
		return err
	}
	if err := runGit(stdout, stderr, workDir, "checkout", branch); err != nil {
		return err
	}
	if err := runGit(stdout, stderr, workDir, "reset", "--hard", "origin/"+branch); err != nil {
		return err
	}
	return runGit(stdout, stderr, workDir, "clean", "-fd")
}

// Commit stages all changes in workDir and creates a commit with the given
// message. Returns true if a commit was created, false if there was nothing
// to commit.
func Commit(workDir, message string, opt Options) (bool, error) {
	stdout, stderr := opt.writers()
	if err := runGit(stdout, stderr, workDir, "add", "-A"); err != nil {
		return false, err
	}
	clean, err := isClean(workDir)
	if err != nil {
		return false, err
	}
	if clean {
		return false, nil
	}
	if err := runGit(stdout, stderr, workDir, "commit", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

// Push pushes the current branch in workDir to its upstream.
func Push(workDir string, opt Options) error {
	stdout, stderr := opt.writers()
	return runGit(stdout, stderr, workDir, "push")
}

// DefaultBranch returns the name of the remote's default branch (e.g. "main"
// or "master") as recorded in refs/remotes/origin/HEAD.
func DefaultBranch(workDir string) (string, error) {
	if out, err := gitOutput(workDir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		// Returns e.g. "origin/main".
		if i := strings.IndexByte(out, '/'); i >= 0 {
			return out[i+1:], nil
		}
		return out, nil
	}
	out, err := gitOutput(workDir, "remote", "show", "origin")
	if err != nil {
		return "", fmt.Errorf("determining default branch: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "HEAD branch:"); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", errors.New("could not determine default branch")
}

func runGit(stdout, stderr io.Writer, dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func isClean(dir string) (bool, error) {
	out, err := gitOutput(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}
