package cli

import (
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/youtube"
)

type mockSource struct {
	videos func() ([]youtube.Video, error)
}

func (m mockSource) Videos() ([]youtube.Video, error) { return m.videos() }

func clearYouTubeEnv(t *testing.T) {
	for _, k := range []string{"YOUTUBE_API_KEY", "YOUTUBE_CHANNEL_ID", "YOUTUBE_FEED_URL", "YOUTUBE_MAX_PAGES"} {
		t.Setenv(k, "")
	}
}

func TestYouTubeCmd_SyncsSource(t *testing.T) {
	clearYouTubeEnv(t)
	old, oldSettings := processYouTubeVideos, settings
	defer func() { processYouTubeVideos, settings = old, oldSettings }()
	settings.YouTubeSource = mockSource{func() ([]youtube.Video, error) { return []youtube.Video{{Title: "yt-test"}}, nil }}
	var gotFolder string
	processYouTubeVideos = func(v []youtube.Video, folder string) (youtube.Result, error) {
		gotFolder = folder
		if len(v) != 1 || v[0].Title != "yt-test" {
			t.Errorf("unexpected videos: %+v", v)
		}
		return youtube.Result{Added: 1}, nil
	}

	cmd := NewYouTubeCmd()
	cmd.Flags().String("data", "./data", "Data folder")
	cmd.SetArgs([]string{"--data", "./testdata"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotFolder != "./testdata" {
		t.Errorf("folder = %q", gotFolder)
	}
}

func TestYouTubeSourceSelection(t *testing.T) {
	old := settings
	defer func() { settings = old }()
	settings = Options{}

	if _, err := youtubeSource(autoconfig.YouTubeSettings{}); err == nil {
		t.Error("expected an error when nothing is configured")
	}
	if _, err := youtubeSource(autoconfig.YouTubeSettings{APIKey: "k"}); err == nil {
		t.Error("an API key without channel must fail")
	}
	src, err := youtubeSource(autoconfig.YouTubeSettings{ChannelID: "UCx"})
	if rss, ok := src.(*youtube.RSSSource); err != nil || !ok || rss.FeedURL != youtube.FeedURL("UCx") {
		t.Errorf("without key the feed is used: %#v %v", src, err)
	}
	src, err = youtubeSource(autoconfig.YouTubeSettings{APIKey: "k", ChannelID: "UCx", MaxPages: 2})
	if api, ok := src.(*youtube.APISource); err != nil || !ok || api.ChannelID != "UCx" || api.MaxPages != 2 {
		t.Errorf("with key the API is used: %#v %v", src, err)
	}
	settings.YouTubeChannelID = "UCdefault"
	src, _ = youtubeSource(autoconfig.YouTubeSettings{APIKey: "k"})
	if api, ok := src.(*youtube.APISource); !ok || api.ChannelID != "UCdefault" {
		t.Errorf("default channel not used: %#v", src)
	}
}
