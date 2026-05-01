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
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "npub"), nil
}

// WorkDir returns the working directory for a given deploy repo URL.
func WorkDir(repoURL string) (string, error) {
	if strings.TrimSpace(repoURL) == "" {
		return "", errors.New("deploy_repo is empty")
	}
	root, err := CacheRoot()
	if err != nil {
		return "", err
	}
	slug := RepoSlug(repoURL)
	if slug == "" {
		return "", fmt.Errorf("cannot derive a directory name from deploy_repo %q", repoURL)
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
// is verified, fetched, and the working copy is hard-reset to the remote
// default branch, discarding local commits and untracked files so the next
// build starts from a clean state.
func Prepare(repoURL, workDir string, opt Options) error {
	if err := requireGit(); err != nil {
		return err
	}
	stdout, stderr := opt.writers()
	info, err := os.Stat(workDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("checking %s: %w", workDir, err)
		}
		if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
			return fmt.Errorf("creating cache parent %s: %w", filepath.Dir(workDir), err)
		}
		if err := runGit(stdout, stderr, "", "clone", repoURL, workDir); err != nil {
			_ = os.RemoveAll(workDir)
			return fmt.Errorf("cloning %s: %w", repoURL, err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("deploy cache path %s exists but is not a directory; remove it and rerun", workDir)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err != nil {
		return fmt.Errorf("deploy cache %s is not a git working copy; remove it and rerun", workDir)
	}
	if err := verifyOrigin(workDir, repoURL); err != nil {
		return err
	}
	if err := runGit(stdout, stderr, workDir, "fetch", "--prune", "origin"); err != nil {
		return fmt.Errorf("fetching %s: %w", repoURL, err)
	}
	branch, err := DefaultBranch(workDir)
	if err != nil {
		return err
	}
	if err := runGit(stdout, stderr, workDir, "checkout", branch); err != nil {
		return fmt.Errorf("checking out %s: %w", branch, err)
	}
	if err := runGit(stdout, stderr, workDir, "reset", "--hard", "origin/"+branch); err != nil {
		return fmt.Errorf("resetting to origin/%s: %w", branch, err)
	}
	if err := runGit(stdout, stderr, workDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("cleaning %s: %w", workDir, err)
	}
	return nil
}

// Commit stages all changes in workDir and creates a commit with the given
// message. Returns true if a commit was created, false if there was nothing
// to commit.
func Commit(workDir, message string, opt Options) (bool, error) {
	if err := requireGit(); err != nil {
		return false, err
	}
	stdout, stderr := opt.writers()
	if err := runGit(stdout, stderr, workDir, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}
	clean, err := isClean(workDir)
	if err != nil {
		return false, fmt.Errorf("checking working tree status: %w", err)
	}
	if clean {
		return false, nil
	}
	if err := runGit(stdout, stderr, workDir, "commit", "-m", message); err != nil {
		return false, fmt.Errorf("creating commit: %w", err)
	}
	return true, nil
}

// Push pushes the current branch in workDir to origin, setting upstream so
// that the first push against a previously-empty repository succeeds.
func Push(workDir string, opt Options) error {
	if err := requireGit(); err != nil {
		return err
	}
	stdout, stderr := opt.writers()
	if err := runGit(stdout, stderr, workDir, "push", "--set-upstream", "origin", "HEAD"); err != nil {
		return fmt.Errorf("pushing to origin: %w", err)
	}
	return nil
}

// DefaultBranch returns the name of the remote's default branch (e.g. "main"
// or "master") as recorded in refs/remotes/origin/HEAD, falling back to the
// currently checked-out local branch if the symbolic ref is absent (e.g. on
// a freshly-cloned empty repository).
func DefaultBranch(workDir string) (string, error) {
	if out, err := gitOutput(workDir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		// Returns e.g. "origin/main".
		if i := strings.IndexByte(out, '/'); i >= 0 {
			return out[i+1:], nil
		}
		return out, nil
	}
	if out, err := gitOutput(workDir, "remote", "show", "origin"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if rest, ok := strings.CutPrefix(line, "HEAD branch:"); ok {
				name := strings.TrimSpace(rest)
				if name != "" && name != "(unknown)" {
					return name, nil
				}
			}
		}
	}
	if out, err := gitOutput(workDir, "symbolic-ref", "--short", "HEAD"); err == nil && out != "" {
		return out, nil
	}
	return "", errors.New("could not determine default branch of origin (is the remote empty?)")
}

func verifyOrigin(workDir, repoURL string) error {
	got, err := gitOutput(workDir, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("reading origin URL of %s: %w", workDir, err)
	}
	if got != repoURL {
		return fmt.Errorf(
			"deploy cache %s tracks %s but deploy_repo is %s; remove the cache directory or revert deploy_repo",
			workDir, got, repoURL,
		)
	}
	return nil
}

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git executable not found in PATH; install git to use npub deploy")
	}
	return nil
}

// runGit executes git, streaming output to the caller's stdout/stderr while
// also capturing stderr so the returned error includes git's own message
// instead of a bare "exit status N".
func runGit(stdout, stderr io.Writer, dir string, args ...string) error {
	var errBuf bytes.Buffer
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &errBuf)
	if err := cmd.Run(); err != nil {
		if msg := lastNonEmptyLine(errBuf.String()); msg != "" {
			return fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
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
		if msg := lastNonEmptyLine(errBuf.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out.String()), nil
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

func isClean(dir string) (bool, error) {
	out, err := gitOutput(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}
