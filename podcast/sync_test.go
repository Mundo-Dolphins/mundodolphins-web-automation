package podcast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessEpisodesRSS_DeduplicatesByLinkAndPrefersFeedAudio(t *testing.T) {
	tempDir := t.TempDir()
	st := Storage{}

	existing := []Episode{
		{
			Title:       "Existing",
			DateAndTime: "2026-03-10T18:30:00Z",
			Link:        "https://example.com/existing",
			Audio:       "https://example.com/listen_mn_existing_1.mp3",
		},
	}
	if err := st.SaveSeason(9, existing, tempDir); err != nil {
		t.Fatalf("failed to create active season file: %v", err)
	}

	incoming := []Episode{
		{
			Title:       "Duplicated episode",
			DateAndTime: "2026-03-12T10:00:00Z",
			Link:        "https://example.com/new",
			Audio:       "https://example.com/listen_mn_123_1.mp3",
		},
		{
			Title:       "Duplicated episode",
			DateAndTime: "2026-03-12T10:02:00Z",
			Link:        "https://example.com/new",
			Audio:       "https://example.com/audio_mf_123_feed_1.mp3",
		},
	}

	if err := Sync(incoming, tempDir, SyncOptions{Since: "2026-03-10T18:30:00Z", AllSeasons: false, PreferAudio: testPreferAudio}); err != nil {
		t.Fatalf("processEpisodesRSS failed: %v", err)
	}

	episodes := readSeasonFile(t, filepath.Join(tempDir, "season_9.json"))
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes in active season, got %d", len(episodes))
	}

	newEpisodeIdx := -1
	for idx, ep := range episodes {
		if ep.Link == "https://example.com/new" {
			newEpisodeIdx = idx
			break
		}
	}
	if newEpisodeIdx == -1 {
		t.Fatal("expected merged episode to exist")
	}
	if episodes[newEpisodeIdx].Audio != "https://example.com/audio_mf_123_feed_1.mp3" {
		t.Fatalf("expected feed audio url, got %s", episodes[newEpisodeIdx].Audio)
	}
}

func TestProcessEpisodesRSS_IgnoresEpisodesBeforeSinceWithoutWriting(t *testing.T) {
	tempDir := t.TempDir()
	st := Storage{}

	existing := []Episode{
		{
			Title:       "Existing",
			DateAndTime: "2026-03-12T08:00:00Z",
			Link:        "https://example.com/existing",
			Audio:       "https://example.com/listen_mn_existing_1.mp3",
		},
	}
	if err := st.SaveSeason(9, existing, tempDir); err != nil {
		t.Fatalf("failed to create active season file: %v", err)
	}

	seasonPath := filepath.Join(tempDir, "season_9.json")
	beforeInfo, err := os.Stat(seasonPath)
	if err != nil {
		t.Fatalf("stat before failed: %v", err)
	}

	incoming := []Episode{
		{
			Title:       "Old episode",
			DateAndTime: "2026-03-11T08:00:00Z",
			Link:        "https://example.com/old",
			Audio:       "https://example.com/listen_mn_old_1.mp3",
		},
	}

	if err := Sync(incoming, tempDir, SyncOptions{Since: "2026-03-12T09:00:00Z", AllSeasons: false, PreferAudio: testPreferAudio}); err != nil {
		t.Fatalf("processEpisodesRSS failed: %v", err)
	}

	afterInfo, err := os.Stat(seasonPath)
	if err != nil {
		t.Fatalf("stat after failed: %v", err)
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("expected no file write when there are no new episodes")
	}

	episodes := readSeasonFile(t, seasonPath)
	if len(episodes) != 1 {
		t.Fatalf("expected existing episodes to remain unchanged, got %d", len(episodes))
	}
}

func TestProcessEpisodesRSS_WithoutSinceAndWithoutJSONsProcessesAll(t *testing.T) {
	tempDir := t.TempDir()

	incoming := []Episode{
		{
			Title:       "Season 8 episode",
			DateAndTime: "2025-08-01T12:00:00Z",
			Link:        "https://example.com/s8",
			Audio:       "https://example.com/listen_mn_s8_1.mp3",
		},
		{
			Title:       "Season 9 episode",
			DateAndTime: "2026-03-12T12:00:00Z",
			Link:        "https://example.com/s9",
			Audio:       "https://example.com/listen_mn_s9_1.mp3",
		},
	}

	if err := Sync(incoming, tempDir, SyncOptions{Since: "", AllSeasons: false, PreferAudio: testPreferAudio}); err != nil {
		t.Fatalf("processEpisodesRSS failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "season_8.json")); err != nil {
		t.Fatalf("expected season_8.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "season_9.json")); err != nil {
		t.Fatalf("expected season_9.json to exist: %v", err)
	}
}

func readSeasonFile(t *testing.T, filePath string) []Episode {
	t.Helper()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed reading season file: %v", err)
	}

	var episodes []Episode
	if err := json.Unmarshal(content, &episodes); err != nil {
		t.Fatalf("failed decoding season file: %v", err)
	}

	return episodes
}

func TestProcessEpisodesRSS_AllSeasonsUpdatesClosedSeason(t *testing.T) {
	tempDir := t.TempDir()
	st := Storage{}

	season8 := []Episode{
		{
			Title:       "Season 8 existing",
			DateAndTime: "2025-08-01T12:00:00Z",
			Link:        "https://example.com/s8-ep",
			Audio:       "https://example.com/listen_mn_s8_1.mp3",
		},
	}
	season9 := []Episode{
		{
			Title:       "Season 9 existing",
			DateAndTime: "2026-03-12T12:00:00Z",
			Link:        "https://example.com/s9-ep",
			Audio:       "https://example.com/listen_mn_s9_1.mp3",
		},
	}

	if err := st.SaveSeason(8, season8, tempDir); err != nil {
		t.Fatalf("failed to create season_8 file: %v", err)
	}
	if err := st.SaveSeason(9, season9, tempDir); err != nil {
		t.Fatalf("failed to create season_9 file: %v", err)
	}

	incoming := []Episode{
		{
			Title:       "Season 8 existing",
			DateAndTime: "2025-08-01T12:01:00Z",
			Link:        "https://example.com/s8-ep",
			Audio:       "https://example.com/audio_mf_s8_feed_1.mp3",
		},
	}

	if err := Sync(incoming, tempDir, SyncOptions{Since: "2025-07-01T00:00:00Z", AllSeasons: true, PreferAudio: testPreferAudio}); err != nil {
		t.Fatalf("processEpisodesRSS failed: %v", err)
	}

	updatedSeason8 := readSeasonFile(t, filepath.Join(tempDir, "season_8.json"))
	if len(updatedSeason8) != 1 {
		t.Fatalf("expected 1 episode in season_8, got %d", len(updatedSeason8))
	}
	if updatedSeason8[0].Audio != "https://example.com/audio_mf_s8_feed_1.mp3" {
		t.Fatalf("expected season_8 audio to be upgraded, got %s", updatedSeason8[0].Audio)
	}
}

func TestProcessEpisodesRSS_CreatesNewSeasonWhenIncomingBelongsToNextSeason(t *testing.T) {
	tempDir := t.TempDir()
	st := Storage{}

	season8 := []Episode{
		{
			Title:       "Season 8 latest",
			DateAndTime: "2026-03-11T20:40:50Z",
			Link:        "https://example.com/s8-latest",
			Audio:       "https://example.com/listen_mn_s8_latest_1.mp3",
		},
	}

	if err := st.SaveSeason(8, season8, tempDir); err != nil {
		t.Fatalf("failed to create season_8 file: %v", err)
	}

	incoming := []Episode{
		{
			Title:       "Season 9 opener",
			DateAndTime: "2026-04-29T17:11:10Z",
			Link:        "https://example.com/s9-opener",
			Audio:       "https://example.com/audio_mf_s9_feed_1.mp3",
		},
	}

	if err := Sync(incoming, tempDir, SyncOptions{Since: "", AllSeasons: false, PreferAudio: testPreferAudio}); err != nil {
		t.Fatalf("processEpisodesRSS failed: %v", err)
	}

	updatedSeason8 := readSeasonFile(t, filepath.Join(tempDir, "season_8.json"))
	if len(updatedSeason8) != 1 {
		t.Fatalf("expected season_8 to remain with 1 episode, got %d", len(updatedSeason8))
	}
	if updatedSeason8[0].Link != "https://example.com/s8-latest" {
		t.Fatalf("expected season_8 to remain unchanged, got %s", updatedSeason8[0].Link)
	}

	season9Path := filepath.Join(tempDir, "season_9.json")
	if _, err := os.Stat(season9Path); err != nil {
		t.Fatalf("expected season_9.json to be created: %v", err)
	}

	createdSeason9 := readSeasonFile(t, season9Path)
	if len(createdSeason9) != 1 {
		t.Fatalf("expected season_9 to have 1 episode, got %d", len(createdSeason9))
	}
	if createdSeason9[0].Link != "https://example.com/s9-opener" {
		t.Fatalf("expected season_9 episode link to be preserved, got %s", createdSeason9[0].Link)
	}
}

// testPreferAudio mimics a platform whose feed URLs are better than page URLs.
func testPreferAudio(url string) bool {
	return strings.Contains(url, "_mf_") && strings.HasSuffix(url, "_feed_1.mp3")
}
