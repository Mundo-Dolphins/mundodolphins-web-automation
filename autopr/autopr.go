package autopr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/filelock"
)

type Git interface {
	AbortInterruptedOperation(ctx context.Context) error
	FetchOrigin(ctx context.Context) error
	DeleteGoneBranches(ctx context.Context) error
	CheckoutWorkBranch(ctx context.Context, workBranch string, baseBranch string) error
	Add(ctx context.Context, pathspecs ...string) error
	Commit(ctx context.Context, message string) (bool, error)
	BranchHasChangesFrom(ctx context.Context, baseRef string) (bool, error)
	Push(ctx context.Context, branch string) error
}

type GitHub interface {
	EnsurePullRequest(ctx context.Context, title string, body string, head string, base string, label string) (PullRequest, bool, error)
}

type PullRequest struct {
	Number  int
	HTMLURL string
}

type Generator func(ctx context.Context) error

var ErrNoBranchChanges = errors.New("work branch has no changes from base branch")

type Service struct {
	Config        autoconfig.Config
	LockPath      string
	Git           Git
	GitHub        GitHub
	Generate      Generator
	Pathspec      string
	CommitMessage string
	PRTitle       string
	PRBody        string
	PRLabel       string
}

type Result struct {
	Locked      bool
	Committed   bool
	PRCreated   bool
	PullRequest PullRequest
}

func (s Service) Run(ctx context.Context) (result Result, runErr error) {
	log.WithField("branch", s.Config.WorkBranch).Info("autopr start")
	lockPath := s.LockPath
	if lockPath == "" {
		lockPath = filepath.Join(s.Config.RepoPath, ".autopr.lock")
	}
	lock := filelock.New(lockPath)
	acquired, err := lock.TryAcquire()
	if err != nil {
		return Result{}, err
	}
	if !acquired {
		return Result{Locked: false}, nil
	}
	defer func() {
		if err := lock.Release(); err != nil {
			runErr = errors.Join(runErr, err)
		}
		if runErr != nil {
			log.WithError(runErr).Error("autopr finished with error")
			return
		}
		log.Info("autopr finished")
	}()

	repoLock := filelock.New(repoLockPath(s.Config.RepoPath))
	repoAcquired, err := repoLock.TryAcquire()
	if err != nil {
		return Result{}, err
	}
	if !repoAcquired {
		log.WithField("repo", s.Config.RepoPath).Info("another autopr execution is already using this repository")
		return Result{Locked: false}, nil
	}
	defer func() {
		if err := repoLock.Release(); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()

	if err := s.Git.AbortInterruptedOperation(ctx); err != nil {
		return Result{}, err
	}
	if err := s.Git.FetchOrigin(ctx); err != nil {
		return Result{}, err
	}
	if err := s.Git.DeleteGoneBranches(ctx); err != nil {
		return Result{}, err
	}
	if err := s.Git.CheckoutWorkBranch(ctx, s.Config.WorkBranch, s.Config.BaseBranch); err != nil {
		return Result{}, err
	}
	log.WithField("base", s.Config.BaseBranch).WithField("branch", s.Config.WorkBranch).Info("repo synchronized")
	if s.Generate != nil {
		if err := s.Generate(ctx); err != nil {
			return Result{}, err
		}
	}
	if err := s.Git.Add(ctx, s.Pathspec); err != nil {
		return Result{}, err
	}
	committed, err := s.Git.Commit(ctx, s.CommitMessage)
	if err != nil {
		return Result{}, err
	}
	if !committed {
		log.Info("no changes")
		hasBranchChanges, err := s.Git.BranchHasChangesFrom(ctx, "origin/"+s.Config.BaseBranch)
		if err != nil {
			return Result{}, err
		}
		if !hasBranchChanges {
			return Result{Locked: true}, nil
		}
		log.WithField("branch", s.Config.WorkBranch).Info("work branch has existing changes")
	}
	if err := s.Config.ValidateGitHub(); err != nil {
		return Result{}, err
	}
	if committed {
		log.Info("commit created")
	}
	if err := s.Git.Push(ctx, s.Config.WorkBranch); err != nil {
		return Result{}, err
	}
	log.WithField("branch", s.Config.WorkBranch).Info("push completed")
	pr, created, err := s.GitHub.EnsurePullRequest(ctx, s.PRTitle, s.PRBody, s.Config.WorkBranch, s.Config.BaseBranch, s.PRLabel)
	if err != nil {
		if errors.Is(err, ErrNoBranchChanges) {
			log.WithField("branch", s.Config.WorkBranch).Info("work branch has no changes after push; pull request was not changed")
			return Result{Locked: true}, runErr
		}
		return Result{}, err
	}
	log.WithField("pr", pr.HTMLURL).WithField("created", created).Info("pull request ready")
	return Result{Locked: true, Committed: true, PRCreated: created, PullRequest: pr}, runErr
}

func repoLockPath(repoPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repoPath)))
	return filepath.Join(os.TempDir(), "autopr-repo-"+hex.EncodeToString(sum[:8])+".lock")
}
