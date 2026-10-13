package autoconfig

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultBaseBranch    = "main"
	DefaultWorkBranch    = "new-social-posts"
	DefaultRSSWorkBranch = "new-podcast-episodes"
	DefaultGitTimeout    = 2 * time.Minute
	DefaultPRLabel       = "automerge"
	DefaultLockPath      = "/tmp/mundo-dolphins-social.lock"
	DefaultRSSLockPath   = "/tmp/mundo-dolphins-rss.lock"
)

type Config struct {
	GitHubOwner string
	GitHubRepo  string
	GitHubToken string
	BaseBranch  string
	WorkBranch  string
	RepoPath    string
	LockPath    string
	GitTimeout  time.Duration
}

func FromEnv() (Config, error) {
	repoPath := os.Getenv("REPO_PATH")
	if repoPath == "" {
		repoPath = os.Getenv("GITHUB_REPO_PATH")
	}
	if repoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, err
		}
		repoPath = wd
	}

	baseBranch := os.Getenv("GITHUB_BASE_BRANCH")
	if baseBranch == "" {
		baseBranch = DefaultBaseBranch
	}

	workBranch := os.Getenv("WORK_BRANCH")
	if workBranch == "" {
		workBranch = DefaultWorkBranch
	}

	return Config{
		GitHubOwner: os.Getenv("GITHUB_OWNER"),
		GitHubRepo:  os.Getenv("GITHUB_REPO"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		BaseBranch:  baseBranch,
		WorkBranch:  workBranch,
		RepoPath:    filepath.Clean(repoPath),
		LockPath:    DefaultLockPath,
		GitTimeout:  DefaultGitTimeout,
	}, nil
}

func FromSocialEnv() (Config, error) {
	repoPath := os.Getenv("SOCIAL_REPOSITORY_PATH")
	if repoPath == "" {
		repoPath = os.Getenv("REPO_PATH")
	}
	if repoPath == "" {
		repoPath = os.Getenv("GITHUB_REPO_PATH")
	}
	if repoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, err
		}
		repoPath = wd
	}

	baseBranch := os.Getenv("SOCIAL_MAIN_BRANCH")
	if baseBranch == "" {
		baseBranch = DefaultBaseBranch
	}

	workBranch := os.Getenv("SOCIAL_WORK_BRANCH")
	if workBranch == "" {
		workBranch = DefaultWorkBranch
	}

	lockPath := os.Getenv("SOCIAL_LOCK_PATH")
	if lockPath == "" {
		lockPath = DefaultLockPath
	}

	return Config{
		GitHubOwner: os.Getenv("GITHUB_OWNER"),
		GitHubRepo:  os.Getenv("GITHUB_REPO"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		BaseBranch:  baseBranch,
		WorkBranch:  workBranch,
		RepoPath:    filepath.Clean(repoPath),
		LockPath:    lockPath,
		GitTimeout:  DefaultGitTimeout,
	}, nil
}

// FromRSSEnv reads the repository settings and RSS_FEED_URL, which is returned
// empty when unset so that the caller can apply its own default.
func FromRSSEnv() (Config, string, error) {
	repoPath := os.Getenv("RSS_REPOSITORY_PATH")
	if repoPath == "" {
		repoPath = os.Getenv("REPO_PATH")
	}
	if repoPath == "" {
		repoPath = os.Getenv("GITHUB_REPO_PATH")
	}
	if repoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, "", err
		}
		repoPath = wd
	}

	baseBranch := os.Getenv("RSS_MAIN_BRANCH")
	if baseBranch == "" {
		baseBranch = DefaultBaseBranch
	}

	workBranch := os.Getenv("RSS_WORK_BRANCH")
	if workBranch == "" {
		workBranch = DefaultRSSWorkBranch
	}

	lockPath := os.Getenv("RSS_LOCK_PATH")
	if lockPath == "" {
		lockPath = DefaultRSSLockPath
	}

	feedURL := os.Getenv("RSS_FEED_URL")

	return Config{
		GitHubOwner: os.Getenv("GITHUB_OWNER"),
		GitHubRepo:  os.Getenv("GITHUB_REPO"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		BaseBranch:  baseBranch,
		WorkBranch:  workBranch,
		RepoPath:    filepath.Clean(repoPath),
		LockPath:    lockPath,
		GitTimeout:  DefaultGitTimeout,
	}, feedURL, nil
}

func (c Config) ValidateGitHub() error {
	if c.GitHubOwner == "" {
		return errors.New("GITHUB_OWNER is required")
	}
	if c.GitHubRepo == "" {
		return errors.New("GITHUB_REPO is required")
	}
	if c.GitHubToken == "" {
		return errors.New("GITHUB_TOKEN is required")
	}
	return nil
}
