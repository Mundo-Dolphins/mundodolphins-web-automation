package gitops

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeExitError int

func (e fakeExitError) Error() string { return "exit" }
func (e fakeExitError) ExitCode() int { return int(e) }

type call struct {
	name string
	args []string
}

type fakeRunner struct {
	calls   []call
	outputs map[string][]byte
	errs    map[string]error
	errSeqs map[string][]error
}

func (r *fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...)})
	key := strings.Join(args, " ")
	if len(r.errSeqs[key]) > 0 {
		err := r.errSeqs[key][0]
		r.errSeqs[key] = r.errSeqs[key][1:]
		return nil, err
	}
	if output, ok := r.outputs[key]; ok {
		return output, r.errs[key]
	}
	return nil, r.errs[key]
}

func TestFetchOriginPrunesDeletedRemoteRefs(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{}}
	client := New("/repo", time.Second, runner)

	if err := client.FetchOrigin(context.Background()); err != nil {
		t.Fatalf("FetchOrigin() error = %v", err)
	}

	want := []string{"fetch", "-p", "origin"}
	if !reflect.DeepEqual(runner.calls[0].args, want) {
		t.Fatalf("fetch args = %#v, want %#v", runner.calls[0].args, want)
	}
}

func TestDeleteGoneBranchesDeletesOnlyNonCurrentGoneBranches(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"branch -vv": []byte(strings.Join([]string{
				"  keep        1234567 [origin/keep] Keep branch",
				"  stale-one   89abcde [origin/stale-one: gone] Old branch",
				"* stale-two   fedcba9 [origin/stale-two: gone] Current branch",
				"  stale-three 7654321 [origin/stale-three: gone] Other old branch",
			}, "\n")),
		},
		errs: map[string]error{},
	}
	client := New("/repo", time.Second, runner)

	if err := client.DeleteGoneBranches(context.Background()); err != nil {
		t.Fatalf("DeleteGoneBranches() error = %v", err)
	}

	want := [][]string{
		{"branch", "-vv"},
		{"branch", "-D", "stale-one"},
		{"branch", "-D", "stale-three"},
	}
	if len(runner.calls) != len(want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
	for i := range want {
		if !reflect.DeepEqual(runner.calls[i].args, want[i]) {
			t.Fatalf("call[%d] args = %#v, want %#v", i, runner.calls[i].args, want[i])
		}
	}
}

func TestCheckoutWorkBranchStartsFromOriginMain(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{}}
	client := New("/repo", time.Second, runner)

	if err := client.CheckoutWorkBranch(context.Background(), "work", "main"); err != nil {
		t.Fatalf("CheckoutWorkBranch() error = %v", err)
	}

	want := []string{"checkout", "-B", "work", "origin/main"}
	if !reflect.DeepEqual(runner.calls[0].args, want) {
		t.Fatalf("checkout args = %#v, want %#v", runner.calls[0].args, want)
	}
}

func TestCommitSkippedWithoutStagedDiff(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{}}
	client := New("/repo", time.Second, runner)

	committed, err := client.Commit(context.Background(), "msg")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if committed {
		t.Fatal("Commit() committed = true, want false")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(runner.calls))
	}
}

func TestCommitAndPushAllowedWithStagedDiff(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{
		"diff --cached --quiet": fakeExitError(1),
	}}
	client := New("/repo", time.Second, runner)

	committed, err := client.Commit(context.Background(), "msg")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if !committed {
		t.Fatal("Commit() committed = false, want true")
	}
	if err := client.Push(context.Background(), "work"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	wantCommit := []string{"commit", "-m", "msg"}
	wantPush := []string{"push", "--force-with-lease", "-u", "origin", "work"}
	if !reflect.DeepEqual(runner.calls[1].args, wantCommit) {
		t.Fatalf("commit args = %#v, want %#v", runner.calls[1].args, wantCommit)
	}
	if !reflect.DeepEqual(runner.calls[2].args, wantPush) {
		t.Fatalf("push args = %#v, want %#v", runner.calls[2].args, wantPush)
	}
}

func TestAbortInterruptedOperationIgnoresCleanGitState(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{
		"rebase --abort": errors.New("fatal: No rebase in progress?"),
		"merge --abort":  errors.New("fatal: There is no merge to abort (MERGE_HEAD missing)."),
	}}
	client := New("/repo", time.Second, runner)

	if err := client.AbortInterruptedOperation(context.Background()); err != nil {
		t.Fatalf("AbortInterruptedOperation() error = %v", err)
	}
}

func TestBranchHasChangesFrom(t *testing.T) {
	runner := &fakeRunner{errs: map[string]error{
		"diff --quiet origin/main...HEAD": fakeExitError(1),
	}}
	client := New("/repo", time.Second, runner)

	hasChanges, err := client.BranchHasChangesFrom(context.Background(), "origin/main")
	if err != nil {
		t.Fatalf("BranchHasChangesFrom() error = %v", err)
	}
	if !hasChanges {
		t.Fatal("BranchHasChangesFrom() = false, want true")
	}

	want := []string{"diff", "--quiet", "origin/main...HEAD"}
	if !reflect.DeepEqual(runner.calls[0].args, want) {
		t.Fatalf("diff args = %#v, want %#v", runner.calls[0].args, want)
	}
}

func TestPushRetriesLimitedTransientFailure(t *testing.T) {
	runner := &fakeRunner{errSeqs: map[string][]error{
		"push --force-with-lease -u origin work": {fakeExitError(128), nil},
	}}
	client := New("/repo", time.Second, runner)
	client.RetryDelay = time.Nanosecond

	if err := client.Push(context.Background(), "work"); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
}

func TestRedactSecrets(t *testing.T) {
	output := "remote: https://x-access-token:ghp_abcdef123456@github.com/owner/repo\nraw ghp_abcdef123456"
	got := redactSecrets(output)
	if strings.Contains(got, "ghp_abcdef123456") {
		t.Fatalf("redactSecrets() leaked token: %s", got)
	}
	if strings.Contains(got, "x-access-token") {
		t.Fatalf("redactSecrets() leaked credential user: %s", got)
	}
}

func TestRefsThatLookLikeOptionsAreRejectedBeforeRunningGit(t *testing.T) {
	bad := []string{"", "--upload-pack=evil", "-f", "a..b", "has space", "tab\there", "x:y", "a~1", "a^", "q?"}
	for _, ref := range bad {
		runner := &fakeRunner{}
		client := New("/repo", time.Second, runner)
		ctx := context.Background()
		calls := []func() error{
			func() error { return client.CheckoutWorkBranch(ctx, ref, "main") },
			func() error { return client.CheckoutWorkBranch(ctx, "work", ref) },
			func() error { return client.Push(ctx, ref) },
			func() error { return client.PullRebase(ctx, "origin", ref) },
			func() error { return client.Rebase(ctx, ref) },
			func() error { _, err := client.BranchHasChangesFrom(ctx, ref); return err },
		}
		for i, call := range calls {
			if err := call(); !errors.Is(err, ErrInvalidRef) {
				t.Errorf("ref %q call %d: expected ErrInvalidRef, got %v", ref, i, err)
			}
		}
		if len(runner.calls) != 0 {
			t.Errorf("ref %q: git must not run, got %v", ref, runner.calls)
		}
	}
}

func TestOrdinaryRefsAreAccepted(t *testing.T) {
	runner := &fakeRunner{}
	client := New("/repo", time.Second, runner)
	if err := client.CheckoutWorkBranch(context.Background(), "new-social-posts", "main"); err != nil {
		t.Fatal(err)
	}
	if err := client.Push(context.Background(), "feature/new-podcast-episodes"); err != nil {
		t.Fatal(err)
	}
}
