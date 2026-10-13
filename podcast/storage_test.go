package podcast_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

func TestReadSeasons(t *testing.T) {
	st := podcast.Storage{}
	// Use a temporary folder for tests
	tempDir := t.TempDir()

	// Create a sample season file
	episodes := []podcast.Episode{{DateAndTime: "2023-03-15T16:00:00Z"}}
	seasonFile := filepath.Join(tempDir, "season_1.json")
	data, _ := json.Marshal(episodes)
	_ = os.WriteFile(seasonFile, data, 0644)

	seasons := st.ReadSeasons(tempDir)
	if len(seasons) == 0 {
		t.Error("Expected non-empty seasons map")
	}
}

func TestSortBySession(t *testing.T) {
	st := podcast.Storage{}
	episodes := []podcast.Episode{
		{
			Title:       "Season 5",
			DateAndTime: "2023-02-15T16:00:00Z",
		},
		{
			Title:       "Season 6",
			DateAndTime: "2024-01-13T16:00:00Z",
		},
		{
			Title:       "Season 8",
			DateAndTime: "2025-08-13T16:00:00Z",
		},
	}
	sortedEpisodes := st.SortBySeason(episodes)

	if len(sortedEpisodes) == 0 {
		t.Error("Expected non-empty sortedEpisodes map")
	}

	if len(sortedEpisodes[5]) != 1 {
		t.Errorf("Expected 1 episode for season 5, got %d", len(sortedEpisodes[5]))
	}
	if len(sortedEpisodes[6]) != 1 {
		t.Errorf("Expected 1 episode for season 5, got %d", len(sortedEpisodes[5]))
	}
	if len(sortedEpisodes[8]) != 1 {
		t.Errorf("Expected 1 episode for season 5, got %d", len(sortedEpisodes[5]))
	}
}

func TestSaveEpisodes(t *testing.T) {
	st := podcast.Storage{}
	tempDir := t.TempDir()

	episodes := map[int][]podcast.Episode{
		1: {
			{DateAndTime: "2023-03-15T16:00:00Z"},
		},
	}

	err := st.SaveEpisodes(episodes, tempDir)
	if err != nil {
		t.Errorf("Failed to save episodes: %v", err)
	}

	// Verify the file was created
	filePath := filepath.Join(tempDir, "season_1.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected season file to be created")
	}
}

func TestSortByDate(t *testing.T) {
	st := podcast.Storage{}
	episodes := []podcast.Episode{
		{DateAndTime: "2023-03-15T16:00:00Z"},
		{DateAndTime: "2024-03-13T16:00:00Z"},
	}
	st.SortByDate(episodes)
	if episodes[0].DateAndTime != "2024-03-13T16:00:00Z" {
		t.Error("Episodes are not sorted by date in descending order")
	}
}

func TestSaveSeason(t *testing.T) {
	st := podcast.Storage{}
	tempDir := t.TempDir()

	episodes := []podcast.Episode{
		{DateAndTime: "2023-03-15T16:00:00Z"},
	}
	err := st.SaveSeason(1, episodes, tempDir)
	if err != nil {
		t.Errorf("Failed to save season: %v", err)
	}

	// Verify the file was created
	filePath := filepath.Join(tempDir, "season_1.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected season file to be created")
	}
}

func TestReadSeasonFile(t *testing.T) {
	st := podcast.Storage{}
	tempDir := t.TempDir()

	episodes := []podcast.Episode{
		{DateAndTime: "2023-03-15T16:00:00Z"},
	}

	// Save the season file
	seasonFile := filepath.Join(tempDir, "season_1.json")
	data, _ := json.Marshal(episodes)
	_ = os.WriteFile(seasonFile, data, 0644)

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp folder: %v", err)
	}

	season, readEpisodes, err := st.ReadSeasonFile(tempDir, files[0])
	if err != nil {
		t.Errorf("Failed to read season file: %v", err)
	}
	if season != 1 {
		t.Errorf("Expected season 1, got %d", season)
	}
	if len(readEpisodes) != 1 {
		t.Errorf("Expected 1 episode, got %d", len(readEpisodes))
	}
}
