package cli

import (
	"errors"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

type mockParser struct {
	podcast.FeedParser
	ParseFunc func(url string) ([]podcast.Episode, error)
}

func (m *mockParser) Parse(url string) ([]podcast.Episode, error) {
	return m.ParseFunc(url)
}

func TestRssCmd_Success(t *testing.T) {
	oldSettings := settings
	settings.FeedURL = "https://feed.example.com/rss.xml"
	defer func() { settings = oldSettings }()
	called := false
	mock := &mockParser{
		ParseFunc: func(url string) ([]podcast.Episode, error) {
			called = true
			return []podcast.Episode{{Title: "rss-test"}}, nil
		},
	}
	cmd := NewRssCmd(mock)
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.Flags().String("since", "", "since")
	cmd.Flags().Bool("all-seasons", false, "all-seasons")
	cmd.SetArgs([]string{"--data", "./testdata", "--since", "2026-03-10T18:30:00Z", "--all-seasons"})
	oldProcess := processRSSEpisodes
	defer func() { processRSSEpisodes = oldProcess }()
	processRSSEpisodes = func(eps []podcast.Episode, folder, since string, allSeasons bool) error {
		if len(eps) != 1 || eps[0].Title != "rss-test" {
			t.Errorf("unexpected episodes: %+v", eps)
		}
		if since != "2026-03-10T18:30:00Z" {
			t.Errorf("unexpected since: %s", since)
		}
		if !allSeasons {
			t.Error("expected allSeasons=true")
		}
		return nil
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando comando: %v", err)
	}
	if !called {
		t.Error("mock Parse was not called")
	}
}

func TestRssCmd_Error(t *testing.T) {
	mock := &mockParser{
		ParseFunc: func(url string) ([]podcast.Episode, error) {
			return nil, errors.New("network failure")
		},
	}
	cmd := NewRssCmd(mock)
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.Flags().String("since", "", "since")
	cmd.Flags().Bool("all-seasons", false, "all-seasons")
	cmd.SetArgs([]string{"--data", "./testdata"})
	oldProcess := processRSSEpisodes
	defer func() { processRSSEpisodes = oldProcess }()
	processRSSEpisodes = func(eps []podcast.Episode, folder, since string, allSeasons bool) error {
		t.Error("processRSSEpisodes should not be called on error")
		return nil
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando comando: %v", err)
	}
}

func TestRssCmd_NoFeedConfiguredDoesNotParse(t *testing.T) {
	oldSettings := settings
	settings.FeedURL = ""
	defer func() { settings = oldSettings }()
	t.Setenv("RSS_FEED_URL", "")

	called := false
	cmd := NewRssCmd(&mockParser{ParseFunc: func(string) ([]podcast.Episode, error) {
		called = true
		return nil, nil
	}})
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.Flags().String("since", "", "since")
	cmd.Flags().Bool("all-seasons", false, "all-seasons")
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("Parse must not be called without a feed URL")
	}
}

func TestRssFeedURLPrefersEnvironment(t *testing.T) {
	oldSettings := settings
	settings.FeedURL = "https://default.example.com/feed"
	defer func() { settings = oldSettings }()

	if got, _ := rssFeedURL("https://env.example.com/feed"); got != "https://env.example.com/feed" {
		t.Fatalf("expected the environment value to win, got %q", got)
	}
	if got, _ := rssFeedURL(""); got != "https://default.example.com/feed" {
		t.Fatalf("expected the configured default, got %q", got)
	}
}
