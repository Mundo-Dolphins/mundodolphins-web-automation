package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	feedTimeout  = 30 * time.Second
	maxBodyBytes = 16 << 20
	userAgent    = "mundodolphins-web-automation (+https://github.com/Mundo-Dolphins/mundodolphins-web-automation)"

	apiBaseURL     = "https://www.googleapis.com/youtube/v3"
	defaultMaxPage = 4
)

// Source reads the videos of a channel, newest first.
type Source interface {
	Videos() ([]Video, error)
}

// RSSSource reads the public Atom feed of a channel. The feed only has the
// latest ~15 videos and neither duration nor embeddable flag, so videos come
// with an empty duration and embeddable true.
type RSSSource struct {
	// FeedURL is the feed to read (see FeedURL).
	FeedURL string
	// Client is used to download the feed; a client with a timeout is used when nil.
	Client *http.Client
}

func (s *RSSSource) Videos() ([]Video, error) {
	body, err := get(s.Client, s.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	feed, err := gofeed.NewParser().ParseString(string(body))
	if err != nil {
		return nil, err
	}

	var videos []Video
	for _, item := range feed.Items {
		id := VideoID(item.Link)
		if id == "" || item.PublishedParsed == nil || item.Title == "" {
			continue
		}
		videos = append(videos, Video{
			PublishedAt: item.PublishedParsed.Format(time.RFC3339),
			Title:       item.Title,
			URL:         "https://www.youtube.com/watch?v=" + id,
			Embeddable:  true,
		})
	}
	return videos, nil
}

// APISource reads the uploads of a channel with the YouTube Data API, which
// gives the duration and whether each video can be embedded. The key is sent
// in a header, so it never shows up in URLs or error messages.
type APISource struct {
	APIKey    string
	ChannelID string
	// MaxPages bounds the pages of 50 uploads that are read (4 when zero).
	MaxPages int
	// BaseURL replaces the API endpoint (for tests).
	BaseURL string
	Client  *http.Client
}

func (s *APISource) Videos() ([]Video, error) {
	if s.APIKey == "" || s.ChannelID == "" {
		return nil, fmt.Errorf("the YouTube API needs an API key and a channel id")
	}
	base := s.BaseURL
	if base == "" {
		base = apiBaseURL
	}
	maxPages := s.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPage
	}
	header := map[string]string{"X-Goog-Api-Key": s.APIKey}
	call := func(endpoint string, params url.Values, out any) error {
		body, err := get(s.Client, base+"/"+endpoint+"?"+params.Encode(), header)
		if err != nil {
			return err
		}
		return json.Unmarshal(body, out)
	}

	var channels struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := call("channels", url.Values{"part": {"contentDetails"}, "id": {s.ChannelID}}, &channels); err != nil {
		return nil, fmt.Errorf("reading the channel: %w", err)
	}
	if len(channels.Items) == 0 || channels.Items[0].ContentDetails.RelatedPlaylists.Uploads == "" {
		return nil, fmt.Errorf("no uploads playlist found for channel %q", s.ChannelID)
	}
	uploads := channels.Items[0].ContentDetails.RelatedPlaylists.Uploads

	var ids []string
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		params := url.Values{"part": {"contentDetails"}, "playlistId": {uploads}, "maxResults": {"50"}}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		var items struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if err := call("playlistItems", params, &items); err != nil {
			return nil, fmt.Errorf("reading the uploads: %w", err)
		}
		for _, it := range items.Items {
			if it.ContentDetails.VideoID != "" {
				ids = append(ids, it.ContentDetails.VideoID)
			}
		}
		if pageToken = items.NextPageToken; pageToken == "" {
			break
		}
	}

	var videos []Video
	for start := 0; start < len(ids); start += 50 {
		chunk := ids[start:min(start+50, len(ids))]
		var details struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title                string `json:"title"`
					PublishedAt          string `json:"publishedAt"`
					LiveBroadcastContent string `json:"liveBroadcastContent"`
				} `json:"snippet"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Status struct {
					Embeddable *bool `json:"embeddable"`
				} `json:"status"`
			} `json:"items"`
		}
		params := url.Values{"part": {"snippet,contentDetails,status"}, "id": {strings.Join(chunk, ",")}}
		if err := call("videos", params, &details); err != nil {
			return nil, fmt.Errorf("reading the video details: %w", err)
		}
		for _, it := range details.Items {
			title := strings.TrimSpace(it.Snippet.Title)
			if it.ID == "" || title == "" || it.Snippet.PublishedAt == "" {
				continue
			}
			if live := it.Snippet.LiveBroadcastContent; live == "live" || live == "upcoming" {
				continue
			}
			videos = append(videos, Video{
				Duration:    formatDuration(it.ContentDetails.Duration),
				PublishedAt: it.Snippet.PublishedAt,
				Title:       title,
				URL:         "https://www.youtube.com/watch?v=" + it.ID,
				Embeddable:  it.Status.Embeddable == nil || *it.Status.Embeddable,
			})
		}
	}
	sort.SliceStable(videos, func(i, j int) bool {
		return publishedTime(videos[i]).After(publishedTime(videos[j]))
	})
	return videos, nil
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// formatDuration converts an ISO 8601 duration to H:MM:SS or M:SS ("" if
// it cannot be read, as for live streams).
func formatDuration(iso string) string {
	m := isoDuration.FindStringSubmatch(iso)
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	sec, _ := strconv.Atoi(m[3])
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, min, sec)
	}
	return fmt.Sprintf("%d:%02d", min, sec)
}

func get(client *http.Client, rawURL string, header map[string]string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: feedTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("YouTube returned HTTP status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}
