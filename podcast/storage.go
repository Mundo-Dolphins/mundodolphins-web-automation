package podcast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Storage struct{}

type Episode struct {
	ImgMain     string `json:"imgMain"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DateAndTime string `json:"dateAndTime"`
	Link        string `json:"link"`
	Audio       string `json:"audio"`
	Length      string `json:"len"`
}

var location, _ = time.LoadLocation("America/New_York")

// SeasonStarts maps a podcast season number to the instant it starts (the NFL
// new league year). Episodes published after a date belong to that season.
// It must be extended every year; replace or extend it before calling Storage.
var SeasonStarts = map[int]time.Time{
	9: time.Date(2026, time.March, 11, 16, 0, 0, 0, location),
	8: time.Date(2025, time.March, 12, 16, 0, 0, 0, location),
	7: time.Date(2024, time.March, 13, 16, 0, 0, 0, location),
	6: time.Date(2023, time.March, 15, 16, 0, 0, 0, location),
	5: time.Date(2022, time.March, 16, 16, 0, 0, 0, location),
	4: time.Date(2021, time.March, 10, 16, 0, 0, 0, location),
	3: time.Date(2020, time.March, 18, 16, 0, 0, 0, location),
	2: time.Date(2019, time.March, 13, 16, 0, 0, 0, location),
	1: time.Date(2018, time.March, 14, 16, 0, 0, 0, location),
}

func (s Storage) ReadSeasons(dataFolder string) map[int][]Episode {
	seasons := make(map[int][]Episode)

	// Read all files in the data folder
	dataFolderPath := filepath.Join(dataFolder)
	files, err := os.ReadDir(dataFolderPath)
	if err != nil {
		return seasons
	}

	for _, file := range files {
		season, episodes, err := s.ReadSeasonFile(dataFolderPath, file)
		if err != nil {
			continue
		}
		// Add the episodes to the seasons map
		seasons[season] = episodes
	}

	return seasons
}

func (s Storage) SortBySeason(episodes []Episode) map[int][]Episode {
	sortedEpisodes := make(map[int][]Episode)

	s.SortByDate(episodes)
	for _, episode := range episodes {
		episodeDate, err := time.Parse("2006-01-02T15:04:05Z", episode.DateAndTime)
		if err != nil {
			continue
		}

		years := make([]int, 0, len(SeasonStarts))
		for year, date := range SeasonStarts {
			if episodeDate.After(date) {
				years = append(years, year)
			}
		}
		selectedYear := findLargestNumber(years)

		sortedEpisodes[selectedYear] = append(sortedEpisodes[selectedYear], episode)
	}

	return sortedEpisodes
}

func (s Storage) SaveEpisodes(episodes map[int][]Episode, dataFolder string) error {
	// Create the data folder if it doesn't exist
	dataFolderPath := filepath.Join(dataFolder)
	if err := os.MkdirAll(dataFolderPath, 0o755); err != nil {
		return err
	}

	for season, eps := range episodes {
		s.SortByDate(eps)

		err := s.SaveSeason(season, eps, dataFolderPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s Storage) SortByDate(eps []Episode) {
	// Sort episodes by DateAndTime in descending order
	sort.Slice(eps, func(i, j int) bool {
		dateI, _ := time.Parse("2006-01-02T15:04:05Z", eps[i].DateAndTime)
		dateJ, _ := time.Parse("2006-01-02T15:04:05Z", eps[j].DateAndTime)
		return dateI.After(dateJ)
	})
}

func (s Storage) SaveSeason(season int, eps []Episode, dataFolder string) error {
	if season == 0 || len(eps) == 0 {
		// No guardar nada si la temporada es 0 o no hay episodios
		return nil
	}
	// Create the JSON file
	filePath := filepath.Join(dataFolder, fmt.Sprintf("season_%d.json", season))
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Encode episodes to JSON and write to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(eps); err != nil {
		return err
	}
	return nil
}

func (s Storage) ReadSeasonFile(dataFolderPath string, file os.DirEntry) (int, []Episode, error) {
	// Parse the season number from the file name
	var season int
	_, _ = fmt.Sscanf(file.Name(), "season_%d.json", &season)

	// Read the file
	filePath := filepath.Join(dataFolderPath, file.Name())
	fileHandle, err := os.Open(filePath)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = fileHandle.Close() }()

	// Decode the JSON file into a slice of episodes
	var episodes []Episode
	decoder := json.NewDecoder(fileHandle)
	if err := decoder.Decode(&episodes); err != nil {
		return 0, nil, err
	}

	return season, episodes, nil
}

func findLargestNumber(nums []int) int {
	if len(nums) == 0 {
		return 1 // handle empty slice case
	}
	largest := nums[0]               // Step 2
	for i := 1; i < len(nums); i++ { // Step 1
		if nums[i] > largest { // Step 3
			largest = nums[i] // Step 4
		}
	}
	return largest // Step 5
}
