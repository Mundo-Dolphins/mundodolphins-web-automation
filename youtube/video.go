// Package youtube keeps the videos of a YouTube channel in the videos.json
// data file of a website, reading the channel's public Atom feed.
package youtube

import (
	"net/url"
	"regexp"
	"strings"
)

// Video is one entry of videos.json. The field order matches the file so that
// rewriting it does not reformat existing entries.
type Video struct {
	Duration    string `json:"duration"`
	IsPodcast   bool   `json:"isPodcast"`
	PublishedAt string `json:"published_at"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Embeddable  bool   `json:"embeddable"`
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// VideoID returns the YouTube video id of a watch or youtu.be URL, ignoring
// any other parameter, or "" when there is none.
func VideoID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	id := u.Query().Get("v")
	if id == "" && u.Host == "youtu.be" {
		id = strings.TrimPrefix(u.Path, "/")
	}
	if !idPattern.MatchString(id) {
		return ""
	}
	return id
}

// FeedURL is the public Atom feed of a channel id (the "UC..." one).
func FeedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + url.QueryEscape(channelID)
}
