package autoconfig

import "testing"

func TestFromYouTubeEnv(t *testing.T) {
	for _, k := range []string{"YOUTUBE_REPOSITORY_PATH", "YOUTUBE_MAIN_BRANCH", "YOUTUBE_WORK_BRANCH", "YOUTUBE_LOCK_PATH", "YOUTUBE_FEED_URL", "YOUTUBE_CHANNEL_ID", "YOUTUBE_API_KEY", "YOUTUBE_MAX_PAGES"} {
		t.Setenv(k, "")
	}
	t.Setenv("REPO_PATH", "/tmp/site")
	t.Setenv("YOUTUBE_CHANNEL_ID", "UCabc")

	cfg, yt, err := FromYouTubeEnv()
	if err != nil {
		t.Fatal(err)
	}
	if yt.FeedURL != "https://www.youtube.com/feeds/videos.xml?channel_id=UCabc" || yt.ChannelID != "UCabc" || yt.APIKey != "" {
		t.Errorf("settings = %+v", yt)
	}
	if cfg.WorkBranch != DefaultYouTubeWorkBranch || cfg.LockPath != DefaultYouTubeLockPath || cfg.RepoPath != "/tmp/site" || cfg.BaseBranch != "main" {
		t.Errorf("unexpected config: %+v", cfg)
	}

	t.Setenv("YOUTUBE_FEED_URL", "https://example.com/feed.xml")
	t.Setenv("YOUTUBE_API_KEY", "key")
	t.Setenv("YOUTUBE_MAX_PAGES", "7")
	if _, yt, _ = FromYouTubeEnv(); yt.FeedURL != "https://example.com/feed.xml" || yt.APIKey != "key" || yt.MaxPages != 7 {
		t.Errorf("settings = %+v", yt)
	}

	t.Setenv("YOUTUBE_MAX_PAGES", "x")
	if _, _, err := FromYouTubeEnv(); err == nil {
		t.Error("an invalid YOUTUBE_MAX_PAGES must fail")
	}
}
