package youtube

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

func TestVideoID(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.youtube.com/watch?v=0BYCsy0fbSo&pp=0gcJCZEKAYcqIYzv": "0BYCsy0fbSo",
		"https://youtu.be/LLtDVyZsiFI":                                    "LLtDVyZsiFI",
		"https://www.youtube.com/watch?v=short":                           "",
		"not a url %%":                                                    "",
	} {
		if got := VideoID(in); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRSSSource(t *testing.T) {
	data, err := os.ReadFile("testdata/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	videos, err := (&RSSSource{FeedURL: srv.URL}).Videos()
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 2 {
		t.Fatalf("got %d videos, want 2", len(videos))
	}
	v := videos[0]
	if v.Title != "Nos invade la jungla hispánica" || v.URL != "https://www.youtube.com/watch?v=LLtDVyZsiFI" ||
		v.PublishedAt != "2026-10-08T17:28:20Z" || !v.Embeddable || v.Duration != "" {
		t.Errorf("unexpected video: %+v", v)
	}
}

func TestRSSSourceHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := (&RSSSource{FeedURL: srv.URL}).Videos(); err == nil {
		t.Fatal("expected an error")
	}
}

func fakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != "secret" || r.URL.Query().Get("key") != "" {
			t.Errorf("the key must travel in the header only: %s", r.URL)
		}
		switch r.URL.Path {
		case "/channels":
			_, _ = w.Write([]byte(`{"items":[{"contentDetails":{"relatedPlaylists":{"uploads":"UUx"}}}]}`))
		case "/playlistItems":
			if r.URL.Query().Get("pageToken") == "" {
				_, _ = w.Write([]byte(`{"nextPageToken":"p2","items":[{"contentDetails":{"videoId":"AAAAAAAAAAA"}}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"items":[{"contentDetails":{"videoId":"BBBBBBBBBBB"}},{"contentDetails":{"videoId":"CCCCCCCCCCC"}}]}`))
		case "/videos":
			_, _ = w.Write([]byte(`{"items":[
			 {"id":"AAAAAAAAAAA","snippet":{"title":" Uno ","publishedAt":"2026-10-01T10:00:00Z","liveBroadcastContent":"none"},"contentDetails":{"duration":"PT1H2M3S"},"status":{"embeddable":true}},
			 {"id":"BBBBBBBBBBB","snippet":{"title":"Dos","publishedAt":"2026-10-08T10:00:00Z","liveBroadcastContent":"none"},"contentDetails":{"duration":"PT24M45S"},"status":{"embeddable":false}},
			 {"id":"CCCCCCCCCCC","snippet":{"title":"Directo","publishedAt":"2026-10-09T10:00:00Z","liveBroadcastContent":"upcoming"},"contentDetails":{"duration":"P0D"},"status":{}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAPISource(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	videos, err := (&APISource{APIKey: "secret", ChannelID: "UCx", BaseURL: srv.URL}).Videos()
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 2 {
		t.Fatalf("got %+v, want 2 videos (the upcoming one is skipped)", videos)
	}
	if videos[0].Title != "Dos" || videos[0].Duration != "24:45" || videos[0].Embeddable {
		t.Errorf("unexpected first video: %+v", videos[0])
	}
	if videos[1].Title != "Uno" || videos[1].Duration != "1:02:03" || !videos[1].Embeddable ||
		videos[1].URL != "https://www.youtube.com/watch?v=AAAAAAAAAAA" {
		t.Errorf("unexpected second video: %+v", videos[1])
	}
}

func TestAPISourceErrorsDoNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	_, err := (&APISource{APIKey: "secret", ChannelID: "UCx", BaseURL: srv.URL}).Videos()
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected an error without the key, got %v", err)
	}
	if _, err := (&APISource{}).Videos(); err == nil {
		t.Fatal("expected an error without key and channel")
	}
}

func TestFormatDuration(t *testing.T) {
	for in, want := range map[string]string{"PT59S": "0:59", "PT5M": "5:00", "PT1H": "1:00:00", "P0D": "", "": ""} {
		if got := formatDuration(in); got != want {
			t.Errorf("formatDuration(%q) = %q, want %q", in, got, want)
		}
	}
}

const storedVideos = `[
  {
    "duration": "59:06",
    "isPodcast": false,
    "published_at": "2026-01-28T11:11:47-08:00",
    "title": "Los Miami ¿Packers?",
    "url": "https://www.youtube.com/watch?v=0BYCsy0fbSo&pp=0gcJCZEKAYcqIYzv",
    "embeddable": true
  }
]`

func TestSync(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "videos.json"), storedVideos)
	write(t, filepath.Join(dir, "season_9.json"), `[
  {
    "imgMain": "",
    "title": "Nos invade la jungla hispánica",
    "description": "a & b",
    "dateAndTime": "2026-10-08T10:00:00Z",
    "link": "https://www.ivoox.com/x_rf_1_1.html",
    "audio": "",
    "len": "1:00:00"
  },
  {
    "imgMain": "",
    "title": "Los Miami ¿Packers?",
    "description": "",
    "dateAndTime": "2026-01-28T18:00:00Z",
    "link": "l2",
    "audio": "",
    "len": ""
  }
]
`)

	feed := []Video{
		{PublishedAt: "2026-10-08T17:28:20Z", Title: "Nos invade la jungla hispánica", URL: "https://www.youtube.com/watch?v=LLtDVyZsiFI", Duration: "1:01:01", Embeddable: false},
		{PublishedAt: "2026-01-28T19:11:47Z", Title: "Los Miami ¿Packers?", URL: "https://www.youtube.com/watch?v=0BYCsy0fbSo", Duration: "59:06", Embeddable: true},
		{PublishedAt: "2026-10-12T10:00:00Z", Title: "Vídeo suelto & más", URL: "https://www.youtube.com/watch?v=AAAAAAAAAAA", Duration: "3:00", Embeddable: true},
	}
	res, err := Sync(feed, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Linked != 2 || res.Updated != 0 {
		t.Fatalf("result = %+v", res)
	}

	got := read(t, filepath.Join(dir, "videos.json"))
	if !strings.Contains(got, `"title": "Vídeo suelto & más"`) {
		t.Errorf("videos.json must not escape HTML:\n%s", got)
	}
	if strings.Index(got, "AAAAAAAAAAA") > strings.Index(got, "LLtDVyZsiFI") ||
		strings.Index(got, "LLtDVyZsiFI") > strings.Index(got, "0BYCsy0fbSo") {
		t.Errorf("videos are not sorted newest first:\n%s", got)
	}
	vs, _ := readVideos(filepath.Join(dir, "videos.json"))
	if vs[0].IsPodcast || !vs[1].IsPodcast || !vs[2].IsPodcast {
		t.Errorf("isPodcast flags wrong (loose, new podcast, old podcast): %+v", vs)
	}

	season := read(t, filepath.Join(dir, "season_9.json"))
	for _, want := range []string{
		`"video": [`,
		`"url": "https://www.youtube.com/watch?v=LLtDVyZsiFI"`,
		`"duration": "1:01:01"`,
		`"embeddable": false`,
		`"url": "https://www.youtube.com/watch?v=0BYCsy0fbSo\u0026pp=0gcJCZEKAYcqIYzv"`, // old video linked too
		`a \u0026 b`, // the Go writer of the season files escapes HTML as the Pi does
	} {
		if !strings.Contains(season, want) {
			t.Errorf("season_9.json lacks %q:\n%s", want, season)
		}
	}

	// Idempotent: a second run changes nothing.
	beforeV, beforeS := got, season
	res, _ = Sync(feed, dir)
	if res != (Result{}) || read(t, filepath.Join(dir, "videos.json")) != beforeV || read(t, filepath.Join(dir, "season_9.json")) != beforeS {
		t.Errorf("second sync changed something: %+v", res)
	}
}

func TestSyncCompletesStoredVideosFromAPI(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "videos.json"), `[
  {
    "duration": "",
    "isPodcast": false,
    "published_at": "2026-10-01T10:00:00Z",
    "title": "Uno",
    "url": "https://www.youtube.com/watch?v=AAAAAAAAAAA",
    "embeddable": true
  }
]`)
	res, err := Sync([]Video{{PublishedAt: "2026-10-01T10:00:00Z", Title: "Uno", URL: "https://www.youtube.com/watch?v=AAAAAAAAAAA", Duration: "5:00", Embeddable: false}}, dir)
	if err != nil || res.Updated != 1 || res.Added != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	vs, _ := readVideos(filepath.Join(dir, "videos.json"))
	if vs[0].Duration != "5:00" || vs[0].Embeddable {
		t.Errorf("stored video not completed: %+v", vs[0])
	}

	// A feed without duration must not erase what the API gave.
	res, _ = Sync([]Video{{PublishedAt: "2026-10-01T10:00:00Z", Title: "Uno", URL: "https://www.youtube.com/watch?v=AAAAAAAAAAA", Embeddable: true}}, dir)
	vs, _ = readVideos(filepath.Join(dir, "videos.json"))
	if res != (Result{}) || vs[0].Duration != "5:00" || vs[0].Embeddable {
		t.Errorf("feed data overwrote API data: res=%+v video=%+v", res, vs[0])
	}
}

func TestSyncKeepsFileFormat(t *testing.T) {
	dir := t.TempDir()
	original := "[\n  {\n    \"duration\": \"1:00\",\n    \"isPodcast\": false,\n    \"published_at\": \"2026-02-02T12:40:21-08:00\",\n    \"title\": \"A\",\n    \"url\": \"https://www.youtube.com/watch?v=BBBBBBBBBBB\",\n    \"embeddable\": false\n  }\n]"
	write(t, filepath.Join(dir, "videos.json"), original)
	res, err := Sync([]Video{{PublishedAt: "2026-10-01T00:00:00Z", Title: "B", URL: "https://www.youtube.com/watch?v=CCCCCCCCCCC", Embeddable: true}}, dir)
	if err != nil || res.Added != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.HasSuffix(read(t, filepath.Join(dir, "videos.json")), original[1:]) {
		t.Errorf("existing entry was reformatted:\n%s", read(t, filepath.Join(dir, "videos.json")))
	}
}

func TestSyncMissingFile(t *testing.T) {
	res, err := Sync([]Video{{PublishedAt: "2026-10-01T00:00:00Z", Title: "B", URL: "https://www.youtube.com/watch?v=CCCCCCCCCCC"}}, t.TempDir())
	if err != nil || res.Added != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestMatchScore(t *testing.T) {
	ep := podcast.Episode{Title: "Mundo Dolphins: Semana 5 previa vs Jets y más", DateAndTime: "2026-10-08T10:00:00Z"}
	if matchScore(Video{Title: "Semana 5 previa vs Jets", PublishedAt: "2026-10-09T10:00:00Z"}, ep) != matchContains {
		t.Error("expected contained title to match")
	}
	if matchScore(Video{Title: "Semana 5 previa vs Jets", PublishedAt: "2026-12-09T10:00:00Z"}, ep) != 0 {
		t.Error("contained title far in time must not match")
	}
	if matchScore(Video{Title: "Otra cosa distinta", PublishedAt: "2026-10-09T10:00:00Z"}, ep) != 0 {
		t.Error("unrelated title must not match")
	}
	exact := podcast.Episode{Title: "Mundo Dolphins: Hola", DateAndTime: "2020-01-01T00:00:00Z"}
	if matchScore(Video{Title: "Hola", PublishedAt: "2026-10-09T10:00:00Z"}, exact) != matchExact {
		t.Error("same title matches regardless of date")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
