package autoconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/youtube"
)

const (
	DefaultBaseBranch        = "main"
	DefaultWorkBranch        = "new-social-posts"
	DefaultRSSWorkBranch     = "new-podcast-episodes"
	DefaultYouTubeWorkBranch = "new-youtube-videos"
	DefaultGitTimeout        = 2 * time.Minute
	DefaultPRLabel           = "automerge"
	DefaultLockPath          = "/tmp/mundo-dolphins-social.lock"
	DefaultRSSLockPath       = "/tmp/mundo-dolphins-rss.lock"
	DefaultYouTubeLockPath   = "/tmp/mundo-dolphins-youtube.lock"
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

// YouTubeSettings says where the channel videos are read from.
type YouTubeSettings struct {
	// APIKey and ChannelID select the YouTube Data API, which gives duration
	// and embeddable flag.
	APIKey    string
	ChannelID string
	// MaxPages bounds the pages of 50 uploads read from the API (0 = default).
	MaxPages int
	// FeedURL is the Atom feed used when there is no API key: YOUTUBE_FEED_URL,
	// else the feed of YOUTUBE_CHANNEL_ID, else empty.
	FeedURL string
}

// FromYouTubeEnv reads the repository settings and the YouTube settings
// (YOUTUBE_API_KEY, YOUTUBE_CHANNEL_ID, YOUTUBE_FEED_URL, YOUTUBE_MAX_PAGES).
func FromYouTubeEnv() (Config, YouTubeSettings, error) {
	repoPath := os.Getenv("YOUTUBE_REPOSITORY_PATH")
	if repoPath == "" {
		repoPath = os.Getenv("REPO_PATH")
	}
	if repoPath == "" {
		repoPath = os.Getenv("GITHUB_REPO_PATH")
	}
	if repoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Config{}, YouTubeSettings{}, err
		}
		repoPath = wd
	}

	baseBranch := os.Getenv("YOUTUBE_MAIN_BRANCH")
	if baseBranch == "" {
		baseBranch = DefaultBaseBranch
	}

	workBranch := os.Getenv("YOUTUBE_WORK_BRANCH")
	if workBranch == "" {
		workBranch = DefaultYouTubeWorkBranch
	}

	lockPath := os.Getenv("YOUTUBE_LOCK_PATH")
	if lockPath == "" {
		lockPath = DefaultYouTubeLockPath
	}

	yt := YouTubeSettings{
		APIKey:    os.Getenv("YOUTUBE_API_KEY"),
		ChannelID: os.Getenv("YOUTUBE_CHANNEL_ID"),
		FeedURL:   os.Getenv("YOUTUBE_FEED_URL"),
	}
	if raw := os.Getenv("YOUTUBE_MAX_PAGES"); raw != "" {
		pages, err := strconv.Atoi(raw)
		if err != nil || pages < 1 {
			return Config{}, YouTubeSettings{}, fmt.Errorf("YOUTUBE_MAX_PAGES must be a positive number, got %q", raw)
		}
		yt.MaxPages = pages
	}
	if yt.FeedURL == "" && yt.ChannelID != "" {
		yt.FeedURL = youtube.FeedURL(yt.ChannelID)
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
	}, yt, nil
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
