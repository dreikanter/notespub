// Package deploy publishes the built site to a git remote. It maintains a
// bare clone of deploy_repo at ~/.cache/npub/<repo>/git and uses the build
// output at ~/.cache/npub/<repo>/build as a temporary work-tree (via git's
// --git-dir and --work-tree options) when committing. This avoids copying
// the site into a separate working copy.
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

// GitDir returns the bare git subdirectory of cacheDir, where `npub deploy`
// clones deploy_repo on first use.
func GitDir(cacheDir string) string {
	return filepath.Join(cacheDir, "git")
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

// Prepare ensures gitDir is a clone of repoURL and resets HEAD + index to
// origin's default branch tip without touching buildDir's contents. On
// first use it runs `git clone --bare`; on subsequent runs it fetches and
// resets. After Prepare returns, the bare repository's index reflects
// origin's last published state, so a subsequent `git add -A` against
// buildDir stages exactly the diff that needs to be committed.
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
	// Refuse to deploy from an empty build directory: with a clean tree
	// reset to origin's last published state, `git add -A` would stage a
	// deletion of every file in origin and commit a wipe-out. Force the
	// user to rebuild instead.
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
		if err := runGit(stdout, stderr, "", "clone", "--bare", repoURL, gitDir); err != nil {
			_ = os.RemoveAll(gitDir)
			return fmt.Errorf("cloning %s: %w", repoURL, err)
		}
		// `git clone --bare` sets up the origin remote but no fetch refspec
		// and no refs/remotes/origin/*. Add the standard refspec so future
		// fetches mirror origin into refs/remotes/origin/*, which lets us
		// distinguish origin's tip from our locally-committed branch.
		if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
			return err
		}
		// Disable bareness so add/commit will operate on --work-tree without
		// refusing.
		if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "config", "core.bare", "false"); err != nil {
			return err
		}
	} else {
		if err := verifyOrigin(gitDir, repoURL); err != nil {
			return err
		}
	}

	if err := ensureGitExclude(gitDir, build.BuildMarkerName); err != nil {
		return err
	}

	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "fetch", "--prune", "origin"); err != nil {
		return fmt.Errorf("fetching %s: %w", repoURL, err)
	}

	branch, err := DefaultBranch(gitDir)
	if err != nil {
		return err
	}
	// reset --mixed updates HEAD and the index, but leaves the work-tree
	// (buildDir) alone so the next `git add -A` sees exactly the difference
	// between origin and the build output.
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "--work-tree="+buildDir,
		"reset", "--mixed", "origin/"+branch); err != nil {
		return fmt.Errorf("resetting to origin/%s: %w", branch, err)
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

// Commit stages every change between origin and buildDir, then commits if
// anything is staged. Returns true when a commit was created.
func Commit(gitDir, buildDir, message string, opt Options) (bool, error) {
	if err := requireGit(); err != nil {
		return false, err
	}
	stdout, stderr := opt.writers()
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "--work-tree="+buildDir, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}
	out, err := gitOutput("", "--git-dir="+gitDir, "--work-tree="+buildDir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("checking status: %w", err)
	}
	if out == "" {
		return false, nil
	}
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "--work-tree="+buildDir, "commit", "-m", message); err != nil {
		return false, fmt.Errorf("creating commit: %w", err)
	}
	return true, nil
}

// Push pushes HEAD to origin, setting upstream so the first push against a
// previously-empty repository succeeds.
func Push(gitDir string, opt Options) error {
	if err := requireGit(); err != nil {
		return err
	}
	stdout, stderr := opt.writers()
	if err := runGit(stdout, stderr, "", "--git-dir="+gitDir, "push", "--set-upstream", "origin", "HEAD"); err != nil {
		return fmt.Errorf("pushing to origin: %w", err)
	}
	return nil
}

// DefaultBranch returns the name of the default branch tracked by gitDir.
// `git clone --bare` initializes HEAD as a symref to refs/heads/<default>,
// so the local symref tells us the branch name without an extra round-trip
// to the remote.
func DefaultBranch(gitDir string) (string, error) {
	if out, err := gitOutput("", "--git-dir="+gitDir, "symbolic-ref", "--short", "HEAD"); err == nil && out != "" {
		return out, nil
	}
	if out, err := gitOutput("", "--git-dir="+gitDir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if i := strings.IndexByte(out, '/'); i >= 0 {
			return out[i+1:], nil
		}
		return out, nil
	}
	return "", errors.New("could not determine default branch (is the remote empty?)")
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
