package podcast

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func serveFeed(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRSSParserParsesEpisodes(t *testing.T) {
	body, err := os.ReadFile("testdata/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	srv := serveFeed(t, http.StatusOK, string(body))

	episodes, err := (&RSSParser{}).Parse(srv.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes (the post without audio is skipped), got %d: %+v", len(episodes), episodes)
	}
	first := episodes[0]
	if first.Title != "Episode 2" || first.Audio != "https://cdn.example.com/ep2.mp3" || first.Length != "00:42:10" ||
		first.Link != "https://podcast.example.com/ep2" || first.DateAndTime != "2026-03-10T18:30:00Z" {
		t.Fatalf("unexpected first episode: %+v", first)
	}
}

func TestRSSParserToleratesMissingITunesExtension(t *testing.T) {
	body, _ := os.ReadFile("testdata/feed.xml")
	srv := serveFeed(t, http.StatusOK, string(body))
	episodes, err := (&RSSParser{}).Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if episodes[1].Length != "" {
		t.Fatalf("expected an empty length without iTunes data, got %q", episodes[1].Length)
	}
}

func TestRSSParserReportsHTTPAndFormatErrors(t *testing.T) {
	if _, err := (&RSSParser{}).Parse(serveFeed(t, http.StatusInternalServerError, "boom").URL); err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
	if _, err := (&RSSParser{}).Parse(serveFeed(t, http.StatusOK, "<html>not a feed</html>").URL); err == nil {
		t.Fatal("expected an error for a non-feed document")
	}
}
