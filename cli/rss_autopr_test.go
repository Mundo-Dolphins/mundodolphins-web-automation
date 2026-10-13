package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

func TestNewCheckerCmdIncludesRSSSubcommand(t *testing.T) {
	cmd := NewCheckerCmd(&podcast.RSSParser{}, social.Reader{})
	if _, _, err := cmd.Find([]string{"rss"}); err != nil {
		t.Fatalf("checker rss command not found: %v", err)
	}
}

func TestNewRSSAutoPRServiceUsesRSSSettings(t *testing.T) {
	tempDir := t.TempDir()
	feedURL := "https://example.com/feed.xml"
	called := false
	mock := &mockParser{
		ParseFunc: func(url string) ([]podcast.Episode, error) {
			called = true
			if url != feedURL {
				t.Fatalf("Parse URL = %q, want %q", url, feedURL)
			}
			return []podcast.Episode{
				{
					Title:       "New episode",
					DateAndTime: "2026-03-12T10:00:00Z",
					Link:        "https://example.com/new",
					Audio:       "https://example.com/audio_mf_123_feed_1.mp3",
				},
			}, nil
		},
	}

	cfg := autoconfig.Config{
		GitHubOwner: "owner",
		GitHubRepo:  "repo",
		GitHubToken: "token",
		BaseBranch:  "main",
		WorkBranch:  "new-podcast-episodes",
		RepoPath:    tempDir,
		LockPath:    filepath.Join(tempDir, "rss.lock"),
	}

	service, err := newRSSAutoPRService(cfg, feedURL, mock)
	if err != nil {
		t.Fatalf("newRSSAutoPRService() error = %v", err)
	}
	if service.LockPath != cfg.LockPath {
		t.Fatalf("LockPath = %q, want %q", service.LockPath, cfg.LockPath)
	}
	if service.Pathspec != "data" {
		t.Fatalf("Pathspec = %q, want data", service.Pathspec)
	}
	if service.CommitMessage != "Added new podcast episodes" {
		t.Fatalf("CommitMessage = %q, want RSS commit message", service.CommitMessage)
	}
	if service.PRTitle != "Added new podcast episodes" {
		t.Fatalf("PRTitle = %q, want RSS PR title", service.PRTitle)
	}
	if service.PRBody != "Automated pull request after adding new podcast episodes" {
		t.Fatalf("PRBody = %q, want RSS PR body", service.PRBody)
	}
	if service.PRLabel != "automerge" {
		t.Fatalf("PRLabel = %q, want automerge", service.PRLabel)
	}

	if err := service.Generate(t.Context()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !called {
		t.Fatal("Parse was not called")
	}

	episodes := readSeasonFile(t, filepath.Join(tempDir, "data", "season_9.json"))
	if len(episodes) != 1 {
		t.Fatalf("expected one episode, got %d", len(episodes))
	}
	if episodes[0].Link != "https://example.com/new" {
		t.Fatalf("episode link = %q, want new episode", episodes[0].Link)
	}
}

func readSeasonFile(t *testing.T, filePath string) []podcast.Episode {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed reading season file: %v", err)
	}
	var episodes []podcast.Episode
	if err := json.Unmarshal(content, &episodes); err != nil {
		t.Fatalf("failed decoding season file: %v", err)
	}
	return episodes
}
