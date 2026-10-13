package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

type Client struct {
	RepoPath   string
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
	Runner     Runner
}

const (
	defaultTimeout    = time.Minute
	defaultRetryCount = 2
	defaultRetryDelay = time.Second
)

func New(repoPath string, timeout time.Duration, runner Runner) *Client {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Client{RepoPath: repoPath, Timeout: timeout, Runner: runner}
}

func (c *Client) FetchOrigin(ctx context.Context) error {
	return c.gitWithRetry(ctx, "fetch", "-p", "origin")
}

// ErrInvalidRef is returned for branch or remote names that could be read by
// git as an option or that are not valid ref names.
var ErrInvalidRef = errors.New("invalid git ref")

func validateRef(refs ...string) error {
	for _, ref := range refs {
		if ref == "" || strings.HasPrefix(ref, "-") || strings.Contains(ref, "..") ||
			strings.ContainsAny(ref, " \t\r\n~^:?*[\\") {
			return fmt.Errorf("%w: %q", ErrInvalidRef, ref)
		}
	}
	return nil
}

func (c *Client) CheckoutWorkBranch(ctx context.Context, workBranch string, baseBranch string) error {
	if err := validateRef(workBranch, baseBranch); err != nil {
		return err
	}
	return c.git(ctx, "checkout", "-B", workBranch, "origin/"+baseBranch)
}

func (c *Client) DeleteGoneBranches(ctx context.Context) error {
	output, err := c.gitOutput(ctx, "branch", "-vv")
	if err != nil {
		return err
	}

	for _, branch := range goneBranches(string(output)) {
		if err := c.git(ctx, "branch", "-D", branch); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) PullRebase(ctx context.Context, remote string, branch string) error {
	if err := validateRef(remote, branch); err != nil {
		return err
	}
	return c.git(ctx, "pull", "--rebase", "--ff-only", remote, branch)
}

func (c *Client) Rebase(ctx context.Context, upstream string) error {
	if err := validateRef(upstream); err != nil {
		return err
	}
	return c.git(ctx, "rebase", upstream)
}

func (c *Client) AbortInterruptedOperation(ctx context.Context) error {
	if err := c.git(ctx, "rebase", "--abort"); err != nil && !isNoRebaseErr(err) {
		return err
	}
	if err := c.git(ctx, "merge", "--abort"); err != nil && !isNoMergeErr(err) {
		return err
	}
	return nil
}

func (c *Client) Add(ctx context.Context, pathspecs ...string) error {
	args := append([]string{"add", "--"}, pathspecs...)
	return c.git(ctx, args...)
}

func (c *Client) HasStagedDiff(ctx context.Context) (bool, error) {
	err := c.git(ctx, "diff", "--cached", "--quiet")
	if err == nil {
		return false, nil
	}
	if exitCode(err) == 1 {
		return true, nil
	}
	return false, err
}

func (c *Client) Commit(ctx context.Context, message string) (bool, error) {
	hasDiff, err := c.HasStagedDiff(ctx)
	if err != nil {
		return false, err
	}
	if !hasDiff {
		return false, nil
	}
	return true, c.git(ctx, "commit", "-m", message)
}

func (c *Client) BranchHasChangesFrom(ctx context.Context, baseRef string) (bool, error) {
	if err := validateRef(baseRef); err != nil {
		return false, err
	}
	err := c.git(ctx, "diff", "--quiet", baseRef+"...HEAD")
	if err == nil {
		return false, nil
	}
	if exitCode(err) == 1 {
		return true, nil
	}
	return false, err
}

func (c *Client) Push(ctx context.Context, branch string) error {
	if err := validateRef(branch); err != nil {
		return err
	}
	return c.gitWithRetry(ctx, "push", "--force-with-lease", "-u", "origin", branch)
}

func isNoRebaseErr(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no rebase in progress")
}

func isNoMergeErr(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no merge to abort") ||
		strings.Contains(message, "merge_head missing")
}

func (c *Client) git(ctx context.Context, args ...string) error {
	_, err := c.gitAttempt(ctx, args...)
	return err
}

func (c *Client) gitWithRetry(ctx context.Context, args ...string) error {
	var err error
	for attempt := 0; attempt <= c.retryCount(); attempt++ {
		_, err = c.gitAttempt(ctx, args...)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil || attempt == c.retryCount() {
			break
		}
		if waitErr := sleep(ctx, c.retryDelay()); waitErr != nil {
			return waitErr
		}
	}
	return err
}

func (c *Client) gitOutput(ctx context.Context, args ...string) ([]byte, error) {
	return c.gitAttempt(ctx, args...)
}

func (c *Client) gitAttempt(ctx context.Context, args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := c.Runner.Run(runCtx, c.RepoPath, "git", args...)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("git %s timed out", args[0])
	}
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w: %s", args[0], err, redactSecrets(string(output)))
	}
	return output, nil
}

func (c *Client) retryCount() int {
	if c.RetryCount > 0 {
		return c.RetryCount
	}
	return defaultRetryCount
}

func (c *Client) retryDelay() time.Duration {
	if c.RetryDelay > 0 {
		return c.RetryDelay
	}
	return defaultRetryDelay
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var (
	tokenPattern    = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]+`)
	basicURLPattern = regexp.MustCompile(`https://([^/\s:@]+:)?[^/\s:@]+@`)
)

func redactSecrets(output string) string {
	output = tokenPattern.ReplaceAllString(output, "<redacted>")
	return basicURLPattern.ReplaceAllString(output, "https://<redacted>@")
}

func exitCode(err error) int {
	var exit interface{ ExitCode() int }
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		return execErr.ExitCode()
	}
	return -1
}

func goneBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.Contains(line, "gone]") {
			continue
		}
		if fields[0] == "*" {
			continue
		}
		branches = append(branches, fields[0])
	}
	return branches
}
