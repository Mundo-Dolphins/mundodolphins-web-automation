package podcast

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"time"
)

// SyncOptions tunes Sync.
type SyncOptions struct {
	// Since, when set (RFC3339), ignores episodes not strictly newer than it.
	// When empty, the newest episode already stored is used.
	Since string
	// AllSeasons rewrites every season instead of only the active one.
	AllSeasons bool
	// PreferAudio reports whether an audio URL is better than another one for
	// the same episode (for example the URL a feed publishes versus a page
	// player). A better incoming URL replaces the stored one. May be nil.
	PreferAudio func(url string) bool
}

// Sync merges episodes read from a feed into the season files of dataFolder
// ("season_N.json"), deduplicating by link.
func Sync(episodes []Episode, dataFolder string, opts SyncOptions) error {
	return syncEpisodes(episodes, dataFolder, opts)
}

func syncEpisodes(episodes []Episode, dataFolder string, opts SyncOptions) error {
	sinceFlag, allSeasons := opts.Since, opts.AllSeasons
	st := Storage{}
	seasonFiles, err := seasonFilesInDataFolder(dataFolder)
	if err != nil {
		return err
	}

	since, err := resolveSince(sinceFlag, st, dataFolder, seasonFiles)
	if err != nil {
		return err
	}

	filtered := filterEpisodesAfter(episodes, since)

	if len(seasonFiles) == 0 {
		bySeason := st.SortBySeason(filtered)
		return st.SaveEpisodes(bySeason, dataFolder)
	}

	if allSeasons {
		return processAllSeasons(st, dataFolder, filtered, opts.PreferAudio)
	}

	bySeason := st.SortBySeason(filtered)
	seasons := st.ReadSeasons(dataFolder)

	for season, incoming := range bySeason {
		if season == 0 || len(incoming) == 0 {
			continue
		}

		merged, changed := mergeEpisodesByLink(seasons[season], incoming, opts.PreferAudio)
		if !changed {
			continue
		}

		st.SortByDate(merged)
		if err := st.SaveSeason(season, merged, dataFolder); err != nil {
			return err
		}
	}

	return nil
}

func processAllSeasons(st Storage, dataFolder string, filtered []Episode, prefer func(string) bool) error {
	seasons := st.ReadSeasons(dataFolder)
	bySeason := st.SortBySeason(filtered)
	changed := false

	for season, incoming := range bySeason {
		if season == 0 || len(incoming) == 0 {
			continue
		}

		merged, seasonChanged := mergeEpisodesByLink(seasons[season], incoming, prefer)
		if seasonChanged {
			seasons[season] = merged
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return st.SaveEpisodes(seasons, dataFolder)
}

func resolveSince(sinceFlag string, st Storage, dataFolder string, seasonFiles []os.DirEntry) (*time.Time, error) {
	if sinceFlag != "" {
		parsed, err := time.Parse(time.RFC3339, sinceFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid --since value %q: %w", sinceFlag, err)
		}
		return &parsed, nil
	}

	if len(seasonFiles) == 0 {
		return nil, nil
	}

	latest, found, err := latestDateInSeasons(st, dataFolder, seasonFiles)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &latest, nil
}

func seasonFilesInDataFolder(dataFolder string) ([]os.DirEntry, error) {
	files, err := os.ReadDir(dataFolder)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	seasonFiles := make([]os.DirEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if _, ok := parseSeasonNumber(file.Name()); ok {
			seasonFiles = append(seasonFiles, file)
		}
	}
	return seasonFiles, nil
}

func latestDateInSeasons(st Storage, dataFolder string, seasonFiles []os.DirEntry) (time.Time, bool, error) {
	var latest time.Time
	found := false

	for _, file := range seasonFiles {
		_, episodes, err := st.ReadSeasonFile(dataFolder, file)
		if err != nil {
			return time.Time{}, false, err
		}

		for _, episode := range episodes {
			episodeDate, err := time.Parse(time.RFC3339, episode.DateAndTime)
			if err != nil {
				continue
			}
			if !found || episodeDate.After(latest) {
				latest = episodeDate
				found = true
			}
		}
	}

	return latest, found, nil
}

func parseSeasonNumber(name string) (int, bool) {
	var season int
	if _, err := fmt.Sscanf(name, "season_%d.json", &season); err != nil {
		return 0, false
	}
	return season, true
}

func filterEpisodesAfter(episodes []Episode, since *time.Time) []Episode {
	if since == nil {
		return episodes
	}

	filtered := make([]Episode, 0, len(episodes))
	for _, episode := range episodes {
		episodeDate, err := time.Parse(time.RFC3339, episode.DateAndTime)
		if err != nil {
			continue
		}
		if episodeDate.After(*since) {
			filtered = append(filtered, episode)
		}
	}
	return filtered
}

func mergeEpisodesByLink(existing, incoming []Episode, prefer func(string) bool) ([]Episode, bool) {
	changed := false

	for _, candidate := range incoming {
		idx := slices.IndexFunc(existing, func(e Episode) bool {
			return e.Link == candidate.Link
		})

		if idx == -1 {
			existing = append(existing, candidate)
			changed = true
			continue
		}

		if prefer != nil && prefer(candidate.Audio) && !prefer(existing[idx].Audio) {
			existing[idx].Audio = candidate.Audio
			changed = true
		}
	}

	return existing, changed
}

// Append adds episodes to every season file of dataFolder, dropping episodes
// that share a publication date with one already stored. It is meant for
// sources that always return the full list (for example a page scraper).
func Append(episodes []Episode, dataFolder string) error {
	st := Storage{}
	seasons := st.ReadSeasons(dataFolder)
	bySeason := st.SortBySeason(episodes)

	for season, eps := range bySeason {
		if season == 0 || len(eps) == 0 {
			continue // Skip season 0 and empty seasons
		}
		seasons[season] = append(seasons[season], eps...)
	}

	return st.SaveEpisodes(removeDuplicateEpisodes(seasons), dataFolder)
}

func removeDuplicateEpisodes(seasons map[int][]Episode) map[int][]Episode {
	seenDates := make(map[string]bool)
	for season, eps := range seasons {
		var uniqueEpisodes []Episode
		for _, episode := range eps {
			if !seenDates[episode.DateAndTime] {
				seenDates[episode.DateAndTime] = true
				uniqueEpisodes = append(uniqueEpisodes, episode)
			}
		}
		seasons[season] = uniqueEpisodes
	}
	return seasons
}
