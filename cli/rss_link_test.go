package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

// A video published before its episode must be attached when the episode is added.
func TestProcessRSSEpisodes_LinksStoredVideos(t *testing.T) {
	dir := t.TempDir()
	videos := `[
  {
    "duration": "54:54",
    "isPodcast": false,
    "published_at": "2026-10-16T10:47:00Z",
    "title": "Descanos ¿Merecido? . Reflexiones inicio temporada",
    "url": "https://www.youtube.com/watch?v=AAAAAAAAAAA",
    "embeddable": true
  }
]`
	if err := os.WriteFile(filepath.Join(dir, "videos.json"), []byte(videos), 0o644); err != nil {
		t.Fatal(err)
	}

	err := processRSSEpisodes([]podcast.Episode{{
		Title:       "Descanos ¿Merecido? . Reflexiones inicio temporada ",
		DateAndTime: "2026-10-16T10:27:13Z",
		Link:        "https://www.ivoox.com/x_rf_1_1.html",
	}}, dir, "", true)
	if err != nil {
		t.Fatal(err)
	}

	season, _ := os.ReadFile(filepath.Join(dir, "season_9.json"))
	if !strings.Contains(string(season), `"url": "https://www.youtube.com/watch?v=AAAAAAAAAAA"`) {
		t.Errorf("the video was not attached to the new episode:\n%s", season)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "videos.json"))
	if !strings.Contains(string(got), `"isPodcast": true`) {
		t.Errorf("the video was not flagged as podcast:\n%s", got)
	}
}

// Without videos.json the episodes are added as before and no file is created.
func TestProcessRSSEpisodes_NoVideosFile(t *testing.T) {
	dir := t.TempDir()
	if err := processRSSEpisodes([]podcast.Episode{{Title: "T", DateAndTime: "2026-10-16T10:27:13Z", Link: "l"}}, dir, "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "videos.json")); err == nil {
		t.Error("videos.json must not be created")
	}
}
