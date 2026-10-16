package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

// VideosFile is the name of the data file inside the data folder.
const VideosFile = "videos.json"

// Result says what Sync changed.
type Result struct {
	// Added are the videos that were not in videos.json.
	Added int
	// Updated are the stored videos whose duration or embeddable flag changed.
	Updated int
	// Linked are the videos newly attached to a podcast episode.
	Linked int
}

// Link links the videos already stored in dataFolder/videos.json to the podcast
// episodes, without reading any channel. Call it after adding episodes: a
// video usually comes out before its episode is in the data, so it is left
// without episode until the episode exists.
func Link(dataFolder string) (Result, error) {
	return Sync(nil, dataFolder)
}

// Sync merges videos into dataFolder/videos.json and links them to the
// podcast episodes of the season_N.json files.
//
//   - A video is identified by its id, so URLs with extra parameters count as
//     the same video. Stored entries are only completed: when the source knows
//     the duration (the API does, the feed does not), the duration and the
//     embeddable flag replace the stored ones.
//   - A video whose title matches a podcast episode is attached to the
//     episode (episode.video) and flagged isPodcast, so the website can show
//     it in the episode instead of the videos list. This is tried for every
//     video, so videos stored before the link existed are linked too.
func Sync(videos []Video, dataFolder string) (Result, error) {
	var res Result
	path := filepath.Join(dataFolder, VideosFile)
	stored, err := readVideos(path)
	if err != nil {
		return res, err
	}
	index := make(map[string]int, len(stored))
	for i, v := range stored {
		index[VideoID(v.URL)] = i
	}

	changed := false
	for _, v := range videos {
		id := VideoID(v.URL)
		if id == "" {
			continue
		}
		i, ok := index[id]
		if !ok {
			index[id] = len(stored)
			stored = append(stored, v)
			res.Added++
			changed = true
			continue
		}
		if v.Duration != "" && (stored[i].Duration != v.Duration || stored[i].Embeddable != v.Embeddable) {
			stored[i].Duration, stored[i].Embeddable = v.Duration, v.Embeddable
			res.Updated++
			changed = true
		}
	}

	linked, flagged, err := linkEpisodes(stored, dataFolder)
	if err != nil {
		return res, err
	}
	res.Linked = linked
	changed = changed || flagged

	if !changed {
		return res, nil
	}
	sort.SliceStable(stored, func(i, j int) bool {
		return publishedTime(stored[i]).After(publishedTime(stored[j]))
	})
	return res, writeVideos(path, stored)
}

// linkEpisodes attaches each video to the episode it matches best, flagging
// the video isPodcast. It reports how many links were added and whether any
// video flag changed.
func linkEpisodes(videos []Video, dataFolder string) (int, bool, error) {
	st := podcast.Storage{}
	seasons := st.ReadSeasons(dataFolder)
	// ReadSeasons also decodes any other .json file (such as videos.json) as
	// season 0; only the season_N.json files are episodes.
	delete(seasons, 0)
	if len(seasons) == 0 {
		return 0, false, nil
	}

	linked, flagged := 0, false
	dirty := map[int]bool{}
	for i := range videos {
		v := &videos[i]
		season, idx, ok := bestEpisode(*v, seasons)
		if !ok {
			continue
		}
		if !v.IsPodcast {
			v.IsPodcast = true
			flagged = true
		}
		ep := &seasons[season][idx]
		if hasVideo(*ep, VideoID(v.URL)) {
			continue
		}
		ep.Video = append(ep.Video, podcast.VideoRef{
			Title:       v.Title,
			Duration:    v.Duration,
			URL:         v.URL,
			PublishedAt: v.PublishedAt,
			Embeddable:  v.Embeddable,
		})
		dirty[season] = true
		linked++
	}

	// Keep the link in step with the video's current duration and flag.
	byID := make(map[string]Video, len(videos))
	for _, v := range videos {
		byID[VideoID(v.URL)] = v
	}
	for season, eps := range seasons {
		for e := range eps {
			for r := range eps[e].Video {
				ref := &eps[e].Video[r]
				v, ok := byID[VideoID(ref.URL)]
				if !ok || (ref.Duration == v.Duration && ref.Embeddable == v.Embeddable) {
					continue
				}
				ref.Duration, ref.Embeddable = v.Duration, v.Embeddable
				dirty[season] = true
			}
		}
	}

	// Oldest first, so the parts of a multi-part episode keep their order.
	for season := range dirty {
		for e := range seasons[season] {
			refs := seasons[season][e].Video
			sort.SliceStable(refs, func(i, j int) bool {
				ti, tj := refTime(refs[i]), refTime(refs[j])
				if !ti.Equal(tj) {
					return ti.Before(tj)
				}
				return refs[i].Title < refs[j].Title
			})
		}
	}

	for season := range dirty {
		if err := st.SaveSeason(season, seasons[season], dataFolder); err != nil {
			return 0, false, err
		}
	}
	return linked, flagged, nil
}

func hasVideo(e podcast.Episode, id string) bool {
	for _, r := range e.Video {
		if VideoID(r.URL) == id {
			return true
		}
	}
	return false
}

func refTime(r podcast.VideoRef) time.Time {
	t, _ := time.Parse(time.RFC3339, r.PublishedAt)
	return t
}

func publishedTime(v Video) time.Time {
	t, _ := time.Parse(time.RFC3339, v.PublishedAt)
	return t
}

func readVideos(path string) ([]Video, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var videos []Video
	if err := json.Unmarshal(data, &videos); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return videos, nil
}

// writeVideos writes the file as the website does: two-space indent, no HTML
// escaping, no trailing newline.
func writeVideos(path string, videos []Video) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(videos); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimRight(buf.Bytes(), "\n"), 0o644)
}

// --- matching videos with episodes -----------------------------------------

var spaces = regexp.MustCompile(`\s+`)

func normalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, prefix := range []string{"Mundo Dolphins:", "Mundo Dolphins"} {
		if strings.HasPrefix(title, prefix) {
			title = strings.TrimSpace(strings.TrimPrefix(title, prefix))
		}
	}
	return spaces.ReplaceAllString(strings.ToLower(title), " ")
}

func datesClose(a, b string, days int) bool {
	ta, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return false
	}
	tb, err := time.Parse(time.RFC3339, b)
	if err != nil {
		return false
	}
	d := ta.Sub(tb)
	if d < 0 {
		d = -d
	}
	return d <= time.Duration(days)*24*time.Hour
}

// Match strength between a video and an episode (0 = no match).
const (
	matchOverlap  = 1 // at least half of the words in common
	matchContains = 2 // one title contains the other
	matchExact    = 3 // same title
)

// matchScore says how well a video fits an episode: the same title always
// matches; a contained title or a half-shared wording only when published
// within three days of each other.
func matchScore(v Video, e podcast.Episode) int {
	vt, et := normalizeTitle(v.Title), normalizeTitle(e.Title)
	if vt == "" || et == "" {
		return 0
	}
	if vt == et {
		return matchExact
	}
	if !datesClose(v.PublishedAt, e.DateAndTime, 3) {
		return 0
	}
	if strings.Contains(et, vt) || strings.Contains(vt, et) {
		return matchContains
	}
	vw, ew := strings.Fields(vt), strings.Fields(et)
	set := make(map[string]bool, len(ew))
	for _, w := range ew {
		set[w] = true
	}
	common, seen := 0, map[string]bool{}
	for _, w := range vw {
		if set[w] && !seen[w] {
			seen[w] = true
			common++
		}
	}
	if float64(common)/float64(max(len(vw), len(ew))) >= 0.5 {
		return matchOverlap
	}
	return 0
}

// bestEpisode finds the episode a video best matches; among equal scores the
// closest in publication date wins.
func bestEpisode(v Video, seasons map[int][]podcast.Episode) (season, idx int, ok bool) {
	best := 0
	var bestGap time.Duration
	vp := publishedTime(v)
	for s, eps := range seasons {
		for i, e := range eps {
			score := matchScore(v, e)
			if score == 0 {
				continue
			}
			ep, _ := time.Parse(time.RFC3339, e.DateAndTime)
			gap := vp.Sub(ep)
			if gap < 0 {
				gap = -gap
			}
			if score > best || (score == best && gap < bestGap) {
				best, bestGap, season, idx, ok = score, gap, s, i, true
			}
		}
	}
	return season, idx, ok
}
