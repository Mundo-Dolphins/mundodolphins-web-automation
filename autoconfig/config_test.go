package autoconfig

import "testing"

func TestFromSocialEnvUsesSocialDefaults(t *testing.T) {
	t.Setenv("SOCIAL_REPOSITORY_PATH", "")
	t.Setenv("REPO_PATH", "")
	t.Setenv("GITHUB_REPO_PATH", "")
	t.Setenv("SOCIAL_WORK_BRANCH", "")
	t.Setenv("SOCIAL_MAIN_BRANCH", "")
	t.Setenv("SOCIAL_LOCK_PATH", "")

	cfg, err := FromSocialEnv()
	if err != nil {
		t.Fatalf("FromSocialEnv() error = %v", err)
	}
	if cfg.WorkBranch != "new-social-posts" {
		t.Fatalf("WorkBranch = %q, want new-social-posts", cfg.WorkBranch)
	}
	if cfg.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q, want main", cfg.BaseBranch)
	}
	if cfg.LockPath != "/tmp/mundo-dolphins-social.lock" {
		t.Fatalf("LockPath = %q, want default social lock", cfg.LockPath)
	}
}

func TestFromSocialEnvUsesSocialOverrides(t *testing.T) {
	t.Setenv("SOCIAL_REPOSITORY_PATH", "/tmp/site")
	t.Setenv("SOCIAL_WORK_BRANCH", "work")
	t.Setenv("SOCIAL_MAIN_BRANCH", "trunk")
	t.Setenv("SOCIAL_LOCK_PATH", "/tmp/custom.lock")

	cfg, err := FromSocialEnv()
	if err != nil {
		t.Fatalf("FromSocialEnv() error = %v", err)
	}
	if cfg.RepoPath != "/tmp/site" {
		t.Fatalf("RepoPath = %q, want /tmp/site", cfg.RepoPath)
	}
	if cfg.WorkBranch != "work" {
		t.Fatalf("WorkBranch = %q, want work", cfg.WorkBranch)
	}
	if cfg.BaseBranch != "trunk" {
		t.Fatalf("BaseBranch = %q, want trunk", cfg.BaseBranch)
	}
	if cfg.LockPath != "/tmp/custom.lock" {
		t.Fatalf("LockPath = %q, want /tmp/custom.lock", cfg.LockPath)
	}
}

func TestFromRSSEnvUsesRSSDefaults(t *testing.T) {
	t.Setenv("RSS_REPOSITORY_PATH", "")
	t.Setenv("REPO_PATH", "")
	t.Setenv("GITHUB_REPO_PATH", "")
	t.Setenv("RSS_WORK_BRANCH", "")
	t.Setenv("RSS_MAIN_BRANCH", "")
	t.Setenv("RSS_LOCK_PATH", "")
	t.Setenv("RSS_FEED_URL", "")

	cfg, feedURL, err := FromRSSEnv()
	if err != nil {
		t.Fatalf("FromRSSEnv() error = %v", err)
	}
	if cfg.WorkBranch != "new-podcast-episodes" {
		t.Fatalf("WorkBranch = %q, want new-podcast-episodes", cfg.WorkBranch)
	}
	if cfg.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q, want main", cfg.BaseBranch)
	}
	if cfg.LockPath != "/tmp/mundo-dolphins-rss.lock" {
		t.Fatalf("LockPath = %q, want default rss lock", cfg.LockPath)
	}
	if feedURL != "" {
		t.Fatalf("feedURL = %q, want empty so the caller applies its default", feedURL)
	}
}

func TestFromRSSEnvUsesRSSOverrides(t *testing.T) {
	t.Setenv("RSS_REPOSITORY_PATH", "/tmp/rss-site")
	t.Setenv("RSS_WORK_BRANCH", "rss-work")
	t.Setenv("RSS_MAIN_BRANCH", "trunk")
	t.Setenv("RSS_LOCK_PATH", "/tmp/rss.lock")
	t.Setenv("RSS_FEED_URL", "https://example.com/feed.xml")

	cfg, feedURL, err := FromRSSEnv()
	if err != nil {
		t.Fatalf("FromRSSEnv() error = %v", err)
	}
	if cfg.RepoPath != "/tmp/rss-site" {
		t.Fatalf("RepoPath = %q, want /tmp/rss-site", cfg.RepoPath)
	}
	if cfg.WorkBranch != "rss-work" {
		t.Fatalf("WorkBranch = %q, want rss-work", cfg.WorkBranch)
	}
	if cfg.BaseBranch != "trunk" {
		t.Fatalf("BaseBranch = %q, want trunk", cfg.BaseBranch)
	}
	if cfg.LockPath != "/tmp/rss.lock" {
		t.Fatalf("LockPath = %q, want /tmp/rss.lock", cfg.LockPath)
	}
	if feedURL != "https://example.com/feed.xml" {
		t.Fatalf("feedURL = %q, want configured feed", feedURL)
	}
}
