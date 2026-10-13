package podcast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	feedTimeout = 30 * time.Second
	// maxFeedBytes bounds how much of a feed is read.
	maxFeedBytes = 16 << 20
	userAgent    = "mundodolphins-web-automation (+https://github.com/Mundo-Dolphins/mundodolphins-web-automation)"
)

// RSSParser reads any standard podcast RSS feed (iTunes extensions included),
// including the feeds that hosting platforms publish for their podcasts.
type RSSParser struct {
	// Client is used to download the feed; a client with a timeout is used when nil.
	Client *http.Client
}

func (p *RSSParser) Parse(url string) ([]Episode, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: feedTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("feed returned HTTP status %d", resp.StatusCode)
	}
	feed, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, err
	}

	var list []Episode
	for _, item := range feed.Items {
		if item.PublishedParsed == nil || len(item.Enclosures) == 0 {
			continue
		}
		imageURL := ""
		if item.Image != nil {
			imageURL = item.Image.URL
		}
		length := ""
		if item.ITunesExt != nil {
			length = item.ITunesExt.Duration
		}
		list = append(list, Episode{
			ImgMain:     imageURL,
			Title:       item.Title,
			Description: item.Description,
			DateAndTime: item.PublishedParsed.Format(time.RFC3339),
			Link:        item.Link,
			Audio:       item.Enclosures[0].URL,
			Length:      length,
		})
	}
	return list, nil
}
