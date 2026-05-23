// Package deploy publishes the built site to a git remote. It keeps a local
// repository at ~/.cache/npub/<repo>/git and uses the build output at
// ~/.cache/npub/<repo>/build as a temporary work-tree (via git's --git-dir and
// --work-tree options), so the site is never copied into a separate working
// copy. It never clones the remote in full: by default it fetches only the
// current branch tip (shallow) to commit onto.
//
// By default a deploy is non-destructive: it appends a commit onto the
// remote's branch tip and pushes normally. With Options.Force it instead
// builds a single root commit from the build output and force-pushes it,
// replacing the remote's history with one revision — useful when the deploy
// repo is pure transport and its history is not worth keeping.
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

	"github.com/dreikanter/npub/internal/build"
)

// CacheRoot returns the root directory used for deploy artifacts.
func CacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "npub"), nil
}

// DefaultCacheDir returns the conventional per-repo cache directory for
// repoURL, ~/.cache/npub/<repo>. Callers may pass any directory to BuildDir
// and GitDir; this is just the default when no override is configured.
func DefaultCacheDir(repoURL string) (string, error) {
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

// BuildDir returns the build subdirectory of cacheDir, where `npub build`
// writes the rendered site. cacheDir is the per-site cache directory
// resolved by the caller.
func BuildDir(cacheDir string) string {
	return filepath.Join(cacheDir, "build")
}

// GitDir returns the git subdirectory of cacheDir, where `npub deploy`
// initializes a local repository on first use.
func GitDir(cacheDir string) string {
	return filepath.Join(cacheDir, "git")
}

// defaultBranch is the branch deploy pushes to when the remote is empty and
// has no default branch of its own yet.
const defaultBranch = "main"

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
	// Force collapses the deploy to a single root commit and force-pushes it,
	// replacing the remote's history. When false (the default), deploy appends
	// a commit onto the remote's current branch tip and pushes normally.
	Force bool
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

// Prepare validates buildDir and ensures gitDir is a local repository wired to
// repoURL as origin. On first use it runs `git init`; on subsequent runs it
// verifies origin still points at repoURL. Unless opt.Force is set, it then
// shallow-fetches the remote's current branch tip and positions HEAD there, so
// the commit Commit builds lands as that tip's child. With opt.Force it skips
// the fetch, leaving HEAD where it is so Commit can build a fresh root commit.
func Prepare(repoURL, gitDir, buildDir string, opt Options) error {
	if err := requireGit(); err != nil {
		return err
	}
	stdout, stderr := opt.writers()

	info, err := os.Stat(buildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("build directory %s does not exist; run `npub build` first", buildDir)
		}
		return fmt.Errorf("checking %s: %w", buildDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("build path %s is not a directory", buildDir)
	}
	// Refuse to deploy from an empty build directory: it would publish a
	// single root commit containing nothing, wiping out the live site. Force
	// the user to rebuild instead.
	empty, err := dirHasNoContent(buildDir)
	if err != nil {
		return err
	}
	if empty {
		return fmt.Errorf("build directory %s is empty; run `npub build` first", buildDir)
	}

	if _, err := os.Stat(gitDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("checking %s: %w", gitDir, err)
		}
		if err := os.MkdirAll(filepath.Dir(gitDir), 0o755); err != nil {
			return fmt.Errorf("creating cache parent %s: %w", filepath.Dir(gitDir), err)
		}
		// Initialize a local repository instead of cloning: deploy only ever
		// writes to origin, so there is nothing worth downloading.
		if err := runGit(stdout, stderr, "", "init", "--bare", gitDir); err != nil {
			_ = os.RemoveAll(gitDir)
			return fmt.Errorf("initializing %s: %w", gitDir, err)
		}
		// Point HEAD at a deterministic branch and disable bareness so
		// add/commit can operate against --work-tree.
		if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "symbolic-ref", "HEAD", "refs/heads/"+defaultBranch); err != nil {
			return err
		}
		if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "config", "core.bare", "false"); err != nil {
			return err
		}
		if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "remote", "add", "origin", repoURL); err != nil {
			return err
		}
	} else {
		if err := verifyOrigin(gitDir, repoURL); err != nil {
			return err
		}
	}

	if err := ensureCommitIdentity(gitDir); err != nil {
		return err
	}
	if err := ensureGitExclude(gitDir, build.BuildMarkerName); err != nil {
		return err
	}

	// In force mode we replace the remote outright, so there is nothing to
	// fetch. Leave HEAD alone; Commit builds a parentless root commit.
	if opt.Force {
		return nil
	}

	// Non-destructive deploy: fetch only the remote branch tip (shallow) and
	// move HEAD onto it, so the commit Commit builds is its child and the push
	// fast-forwards. An empty remote has no branch to fetch; leave HEAD unborn
	// so the first deploy creates the branch.
	branch, exists, err := remoteHead(gitDir)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "fetch", "--depth", "1", "origin", branch); err != nil {
		return fmt.Errorf("fetching origin/%s: %w", branch, err)
	}
	tip, err := gitOutput("", "--git-dir="+gitDir, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return fmt.Errorf("resolving origin/%s: %w", branch, err)
	}
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "update-ref", "HEAD", tip); err != nil {
		return fmt.Errorf("positioning HEAD at origin/%s: %w", branch, err)
	}
	return nil
}

func ensureGitExclude(gitDir, pattern string) error {
	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("creating git exclude directory %s: %w", filepath.Dir(excludePath), err)
	}
	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading git exclude %s: %w", excludePath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	file, err := os.OpenFile(excludePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening git exclude %s: %w", excludePath, err)
	}
	defer func() { _ = file.Close() }()
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("writing git exclude %s: %w", excludePath, err)
		}
	}
	if _, err := fmt.Fprintf(file, "%s\n", pattern); err != nil {
		return fmt.Errorf("writing git exclude %s: %w", excludePath, err)
	}
	return nil
}

// Commit captures the entire contents of buildDir as a new commit and points
// HEAD at it. The index is rebuilt from scratch each time so files removed
// since the last deploy drop out of the published tree. With opt.Force the
// commit is a parentless root (collapsing history to one revision); otherwise
// it is a child of the current HEAD (the remote tip Prepare positioned).
// Returns false (no commit) when the build output matches HEAD's tree, so an
// unchanged build is a no-op rather than an empty push.
func Commit(gitDir, buildDir, message string, opt Options) (bool, error) {
	if err := requireGit(); err != nil {
		return false, err
	}
	stdout, stderr := opt.writers()
	// Clear the index, then stage the whole build output, so the staged tree
	// is exactly buildDir regardless of what the previous deploy left behind.
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "read-tree", "--empty"); err != nil {
		return false, fmt.Errorf("clearing index: %w", err)
	}
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "--work-tree="+buildDir, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}
	tree, err := gitOutput("", "--git-dir="+gitDir, "write-tree")
	if err != nil {
		return false, fmt.Errorf("writing tree: %w", err)
	}

	// commit-tree args: optionally parent the new commit on HEAD. In force mode
	// we omit the parent to produce a root commit.
	args := []string{"--git-dir=" + gitDir, "commit-tree", tree}
	if parent, err := gitOutput("", "--git-dir="+gitDir, "rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
		if prevTree, err := gitOutput("", "--git-dir="+gitDir, "rev-parse", "--verify", "--quiet", "HEAD^{tree}"); err == nil && prevTree == tree {
			return false, nil
		}
		if !opt.Force {
			args = append(args, "-p", parent)
		}
	}
	args = append(args, "-m", message)

	commit, err := gitOutput("", args...)
	if err != nil {
		return false, fmt.Errorf("creating commit: %w", err)
	}
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "update-ref", "HEAD", commit); err != nil {
		return false, fmt.Errorf("updating HEAD: %w", err)
	}
	return true, nil
}

// Push publishes HEAD to origin's default branch. A normal deploy pushes
// fast-forward; with opt.Force it force-pushes, replacing whatever the remote
// held with the single root commit Commit built.
func Push(gitDir string, opt Options) error {
	if err := requireGit(); err != nil {
		return err
	}
	stdout, stderr := opt.writers()
	branch, err := remoteDefaultBranch(gitDir)
	if err != nil {
		return err
	}
	args := []string{"--git-dir=" + gitDir, "push"}
	if opt.Force {
		args = append(args, "--force")
	}
	args = append(args, "origin", "HEAD:refs/heads/"+branch)
	if err := runGit(stdout, stderr, "", args...); err != nil {
		return fmt.Errorf("pushing to origin: %w", err)
	}
	return nil
}

// remoteHead asks origin which branch its HEAD points at and whether the
// remote advertises one at all. deploy pushes to the branch the remote already
// serves (e.g. main or gh-pages) rather than guessing. An empty remote
// advertises no HEAD: exists is false and branch falls back to defaultBranch.
func remoteHead(gitDir string) (branch string, exists bool, err error) {
	out, err := gitOutput("", "--git-dir="+gitDir, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", false, fmt.Errorf("querying remote default branch: %w", err)
	}
	// A non-empty remote prints a line like "ref: refs/heads/main\tHEAD".
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "ref:")
		if !ok {
			continue
		}
		if fields := strings.Fields(rest); len(fields) > 0 {
			return strings.TrimPrefix(fields[0], "refs/heads/"), true, nil
		}
	}
	return defaultBranch, false, nil
}

// remoteDefaultBranch returns the branch deploy should push to, falling back
// to defaultBranch for an empty remote.
func remoteDefaultBranch(gitDir string) (string, error) {
	branch, _, err := remoteHead(gitDir)
	return branch, err
}

// ensureCommitIdentity sets a fallback committer identity on gitDir when none
// is configured, so commit-tree succeeds in environments (such as CI) without
// a global git identity. A configured identity at any scope is left untouched.
func ensureCommitIdentity(gitDir string) error {
	for key, fallback := range map[string]string{"user.email": "npub@localhost", "user.name": "npub"} {
		if _, err := gitOutput("", "--git-dir="+gitDir, "config", key); err == nil {
			continue
		}
		if err := runGit(io.Discard, io.Discard, "", "--git-dir="+gitDir, "config", key, fallback); err != nil {
			return err
		}
	}
	return nil
}

func verifyOrigin(gitDir, repoURL string) error {
	got, err := gitOutput("", "--git-dir="+gitDir, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("reading origin URL of %s: %w", gitDir, err)
	}
	if got != repoURL {
		return fmt.Errorf(
			"deploy cache %s tracks %s but deploy_repo is %s; remove the cache directory or revert deploy_repo",
			gitDir, got, repoURL,
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

// dirHasNoContent reports whether dir contains no entries other than
// dotfiles. Returns true for a directory that is fully empty too.
func dirHasNoContent(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return false, nil
		}
	}
	return true, nil
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
