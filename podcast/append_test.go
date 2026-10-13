package podcast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppend(t *testing.T) {
	tempDir := t.TempDir()

	episodes := []Episode{
		{
			Title:       "Test Episode",
			DateAndTime: "2023-03-15T16:00:00Z",
		},
	}

	err := Append(episodes, tempDir)
	if err != nil {
		t.Errorf("Append failed: %v", err)
	}

	// Verify that a file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected at least one season file to be created")
	}
}

func TestAppendWithExistingSeasons(t *testing.T) {
	tempDir := t.TempDir()

	// Create an existing season file
	existingEpisodes := []Episode{
		{
			Title:       "Existing Episode",
			DateAndTime: "2023-03-16T16:00:00Z",
		},
	}

	st := Storage{}
	err := st.SaveSeason(5, existingEpisodes, tempDir)
	if err != nil {
		t.Fatalf("Failed to save existing season: %v", err)
	}

	// Procesar nuevos episodios
	newEpisodes := []Episode{
		{
			Title:       "New Episode",
			DateAndTime: "2023-03-15T16:00:00Z",
		},
	}

	err = Append(newEpisodes, tempDir)
	if err != nil {
		t.Errorf("Append failed: %v", err)
	}

	// Verify the existing season file was preserved
	seasonFile := filepath.Join(tempDir, "season_5.json")
	if _, err := os.Stat(seasonFile); os.IsNotExist(err) {
		t.Error("Expected existing season file to be preserved")
	}
}
