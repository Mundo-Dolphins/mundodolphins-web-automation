package autopr

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/filelock"
)

type fakeGit struct {
	calls         []string
	commitResult  bool
	branchChanges bool
	pushErr       error
}

func (g *fakeGit) FetchOrigin(ctx context.Context) error {
	g.calls = append(g.calls, "fetch")
	return nil
}

func (g *fakeGit) DeleteGoneBranches(ctx context.Context) error {
	g.calls = append(g.calls, "delete-gone-branches")
	return nil
}

func (g *fakeGit) AbortInterruptedOperation(ctx context.Context) error {
	g.calls = append(g.calls, "abort-interrupted")
	return nil
}

func (g *fakeGit) CheckoutWorkBranch(ctx context.Context, workBranch string, baseBranch string) error {
	g.calls = append(g.calls, "checkout:"+workBranch+":"+baseBranch)
	return nil
}

func (g *fakeGit) Add(ctx context.Context, pathspecs ...string) error {
	g.calls = append(g.calls, "add:"+pathspecs[0])
	return nil
}

func (g *fakeGit) Commit(ctx context.Context, message string) (bool, error) {
	g.calls = append(g.calls, "commit:"+message)
	return g.commitResult, nil
}

func (g *fakeGit) BranchHasChangesFrom(ctx context.Context, baseRef string) (bool, error) {
	g.calls = append(g.calls, "branch-changes:"+baseRef)
	return g.branchChanges, nil
}

func (g *fakeGit) Push(ctx context.Context, branch string) error {
	g.calls = append(g.calls, "push:"+branch)
	return g.pushErr
}

type fakeGitHub struct {
	calls []string
	err   error
}

func (g *fakeGitHub) EnsurePullRequest(ctx context.Context, title string, body string, head string, base string, label string) (PullRequest, bool, error) {
	g.calls = append(g.calls, "pr:"+head+":"+base+":"+label)
	if g.err != nil {
		return PullRequest{}, false, g.err
	}
	return PullRequest{Number: 12, HTMLURL: "https://example/pr/12"}, true, nil
}

func TestRunSkipsPullRequestWhenThereAreNoDataChanges(t *testing.T) {
	git := &fakeGit{commitResult: false}
	github := &fakeGitHub{}
	generated := false

	result, err := Service{
		Config:        testConfig(),
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { generated = true; return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Locked {
		t.Fatal("Locked = false, want true")
	}
	if result.Committed {
		t.Fatal("Committed = true, want false")
	}
	if !generated {
		t.Fatal("Generate was not called")
	}
	if len(github.calls) != 0 {
		t.Fatalf("GitHub calls = %v, want none", github.calls)
	}

	wantGit := []string{
		"abort-interrupted",
		"fetch",
		"delete-gone-branches",
		"checkout:new-social-posts:main",
		"add:data",
		"commit:Added new social posts",
		"branch-changes:origin/main",
	}
	if !reflect.DeepEqual(git.calls, wantGit) {
		t.Fatalf("git calls = %#v, want %#v", git.calls, wantGit)
	}
}

func TestRunPushesAndEnsuresPullRequestWhenDataChanges(t *testing.T) {
	git := &fakeGit{commitResult: true}
	github := &fakeGitHub{}

	result, err := Service{
		Config:        testConfig(),
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Committed {
		t.Fatal("Committed = false, want true")
	}
	if !result.PRCreated {
		t.Fatal("PRCreated = false, want true")
	}
	if result.PullRequest.Number != 12 {
		t.Fatalf("PullRequest.Number = %d, want 12", result.PullRequest.Number)
	}

	wantGitHub := []string{"pr:new-social-posts:main:automerge"}
	if !reflect.DeepEqual(github.calls, wantGitHub) {
		t.Fatalf("GitHub calls = %#v, want %#v", github.calls, wantGitHub)
	}
	for _, call := range git.calls {
		if call == "push:--force" {
			t.Fatal("unexpected force push")
		}
	}
}

func TestRunEnsuresPullRequestWhenWorkBranchAlreadyHasChanges(t *testing.T) {
	git := &fakeGit{commitResult: false, branchChanges: true}
	github := &fakeGitHub{}

	result, err := Service{
		Config:        testConfig(),
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Committed {
		t.Fatal("Committed = false, want true")
	}
	if len(github.calls) != 1 {
		t.Fatalf("GitHub calls = %v, want one call", github.calls)
	}
	if got := git.calls[len(git.calls)-1]; got != "push:new-social-posts" {
		t.Fatalf("last git call = %q, want push:new-social-posts", got)
	}
}

func TestRunDoesNotEnsurePullRequestWhenPushFails(t *testing.T) {
	git := &fakeGit{commitResult: true, pushErr: errors.New("network reset")}
	github := &fakeGitHub{}

	_, err := Service{
		Config:        testConfig(),
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err == nil {
		t.Fatal("expected push error")
	}
	if len(github.calls) != 0 {
		t.Fatalf("GitHub calls = %v, want none", github.calls)
	}
}

func TestRunReturnsCleanlyWhenGitHubReportsNoBranchChanges(t *testing.T) {
	git := &fakeGit{commitResult: true}
	github := &fakeGitHub{err: ErrNoBranchChanges}

	result, err := Service{
		Config:        testConfig(),
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Locked {
		t.Fatal("Locked = false, want true")
	}
	if result.Committed {
		t.Fatal("Committed = true, want false")
	}
	if result.PRCreated {
		t.Fatal("PRCreated = true, want false")
	}

	wantGitHub := []string{"pr:new-social-posts:main:automerge"}
	if !reflect.DeepEqual(github.calls, wantGitHub) {
		t.Fatalf("GitHub calls = %#v, want %#v", github.calls, wantGitHub)
	}
}

func TestRunValidatesGitHubBeforePush(t *testing.T) {
	git := &fakeGit{commitResult: true}
	github := &fakeGitHub{}
	cfg := testConfig()
	cfg.GitHubToken = ""

	_, err := Service{
		Config:        cfg,
		LockPath:      filepath.Join(t.TempDir(), "social.lock"),
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err == nil {
		t.Fatal("expected GitHub config error")
	}
	for _, call := range git.calls {
		if call == "push:new-social-posts" {
			t.Fatal("push should not run without GitHub config")
		}
	}
	if len(github.calls) != 0 {
		t.Fatalf("GitHub calls = %v, want none", github.calls)
	}
}

func TestRunReturnsCleanlyWhenLockIsAlreadyHeld(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "social.lock")
	lock := filelock.New(lockPath)
	acquired, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("TryAcquire() acquired = false, want true")
	}
	defer func() {
		if err := lock.Release(); err != nil {
			t.Fatalf("Release() error = %v", err)
		}
	}()

	git := &fakeGit{commitResult: true}
	github := &fakeGitHub{}
	result, err := Service{
		Config:        testConfig(),
		LockPath:      lockPath,
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { t.Fatal("Generate should not be called"); return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Locked {
		t.Fatal("Locked = true, want false")
	}
	if len(git.calls) != 0 {
		t.Fatalf("git calls = %v, want none", git.calls)
	}
	if len(github.calls) != 0 {
		t.Fatalf("GitHub calls = %v, want none", github.calls)
	}
}

func TestRunReturnsCleanlyWhenRepositoryLockIsAlreadyHeld(t *testing.T) {
	cfg := testConfig()
	lockPath := filepath.Join(t.TempDir(), "social.lock")
	repoLock := filelock.New(repoLockPath(cfg.RepoPath))
	acquired, err := repoLock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("TryAcquire() acquired = false, want true")
	}
	defer func() {
		if err := repoLock.Release(); err != nil {
			t.Fatalf("Release() error = %v", err)
		}
	}()

	git := &fakeGit{commitResult: true}
	github := &fakeGitHub{}
	result, err := Service{
		Config:        cfg,
		LockPath:      lockPath,
		Git:           git,
		GitHub:        github,
		Generate:      func(ctx context.Context) error { t.Fatal("Generate should not be called"); return nil },
		Pathspec:      "data",
		CommitMessage: "Added new social posts",
		PRTitle:       "Added new social posts",
		PRBody:        "body",
		PRLabel:       autoconfig.DefaultPRLabel,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Locked {
		t.Fatal("Locked = true, want false")
	}
	if len(git.calls) != 0 {
		t.Fatalf("git calls = %v, want none", git.calls)
	}
	if len(github.calls) != 0 {
		t.Fatalf("GitHub calls = %v, want none", github.calls)
	}
}

func testConfig() autoconfig.Config {
	return autoconfig.Config{
		GitHubOwner: "owner",
		GitHubRepo:  "repo",
		GitHubToken: "token",
		BaseBranch:  "main",
		WorkBranch:  "new-social-posts",
		RepoPath:    "/repo",
	}
}
