package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/retryhttp"
)

const (
	defaultBlueSkyAuthorFeedEndpoint = "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed"
	blueSkyAuthorFeedFilter          = "posts_no_replies"
	defaultBlueSkyRetryCount         = 2
	defaultBlueSkyTimeout            = 15 * time.Second
)

// maxResponseBytes bounds how much of a response is decoded.
const maxResponseBytes = 8 << 20

var defaultBlueSkyHTTPClient = &http.Client{Timeout: defaultBlueSkyTimeout}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type BlueSkyPost struct {
	BskyURI        string `json:"BskyURI"`
	BskyCID        string `json:"BskyCID"`
	Description    string `json:"Description"`
	BskyProfileURI string `json:"BskyProfileURI"`
	BskyProfile    string `json:"BskyProfile"`
	BskyPost       string `json:"BskyPost"`
}

type oEmbedResponse struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	AuthorName  string `json:"author_name"`
	AuthorURL   string `json:"author_url"`
	ProviderURL string `json:"provider_url"`
	CacheAge    int    `json:"cache_age"`
	Width       int    `json:"width"`
	Height      *int   `json:"height"`
	HTML        string `json:"html"`
}

type blueSkyAuthorFeedResponse struct {
	Feed []blueSkyFeedItem `json:"feed"`
}

type blueSkyFeedItem struct {
	Post blueSkyAPIPost `json:"post"`
}

type blueSkyAPIPost struct {
	URI       string           `json:"uri"`
	CID       string           `json:"cid"`
	Author    blueSkyAPIAuthor `json:"author"`
	Record    blueSkyAPIRecord `json:"record"`
	IndexedAt string           `json:"indexedAt"`
	Embed     blueSkyAPIEmbed  `json:"embed"`
}

type blueSkyAPIAuthor struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type blueSkyAPIRecord struct {
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type blueSkyAPIEmbed struct {
	Images []blueSkyAPIImage `json:"images"`
}

type blueSkyAPIImage struct {
	Alt string `json:"alt"`
}

type Reader struct {
	HTTPClient                httpDoer
	BlueSkyAuthorFeedEndpoint string
	BlueSkyRetryCount         int
	BlueSkyRetryDelay         time.Duration
	UseBlueSkyRSSFallback     bool
	// Instagram, when set, provides the Instagram posts. The library does not
	// implement it (see InstagramSource); without it Instagram is skipped.
	Instagram InstagramSource
}

func (r Reader) fetchBlueSkyFeeds(profiles []string, rssFeeds []string, posts []string) ([]OutputPost, error) {
	profiles = r.blueSkyProfiles(profiles, rssFeeds)
	seenPosts := make(map[string]struct{}, len(posts))
	for _, post := range posts {
		seenPosts[post] = struct{}{}
	}

	var allItems []OutputPost
	var firstErr error
	for _, profile := range profiles {
		items, err := r.fetchBlueSkyAuthorFeed(profile, seenPosts)
		if err != nil && r.UseBlueSkyRSSFallback {
			log.WithField("process", "fetchBlueSkyFeeds").
				WithField("profile", profile).
				Warnf("public Bluesky API failed, trying RSS fallback: %v", err)
			items, err = r.fetchBlueSkyRSSFeed(profile, seenPosts)
		}
		if err != nil {
			log.WithField("process", "fetchBlueSkyFeeds").
				WithField("profile", profile).
				Errorf("error fetching Bluesky profile: %v", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, item := range items {
			seenPosts[item.ID] = struct{}{}
			allItems = append(allItems, item)
		}
	}

	if len(allItems) > 0 {
		return allItems, nil
	}
	return allItems, firstErr
}

func (r Reader) blueSkyProfiles(profiles []string, rssFeeds []string) []string {
	if len(profiles) > 0 {
		return profiles
	}

	derived := make([]string, 0, len(rssFeeds))
	for _, feed := range rssFeeds {
		if profile := blueSkyProfileFromRSSURL(feed); profile != "" {
			derived = append(derived, profile)
		}
	}
	return derived
}

func blueSkyProfileFromRSSURL(feed string) string {
	u, err := url.Parse(feed)
	if err != nil {
		return ""
	}
	const prefix = "/profile/"
	if !strings.HasPrefix(u.Path, prefix) || !strings.HasSuffix(u.Path, "/rss") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(u.Path, prefix), "/rss")
}

func (r Reader) fetchBlueSkyAuthorFeed(profile string, seenPosts map[string]struct{}) ([]OutputPost, error) {
	requestURL, err := url.Parse(r.blueSkyAuthorFeedEndpoint())
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("actor", profile)
	query.Set("filter", blueSkyAuthorFeedFilter)
	requestURL.RawQuery = query.Encode()

	log.WithField("process", "fetchBlueSkyAuthorFeed").
		WithField("profile", profile).
		Debug("fetching Bluesky public API")

	resp, err := r.doBlueSkyRequest(requestURL.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var feed blueSkyAuthorFeedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&feed); err != nil {
		return nil, err
	}

	items := make([]OutputPost, 0, len(feed.Feed))
	for _, item := range feed.Feed {
		outputPost, err := mapBlueSkyAPIPost(item.Post, profile)
		if err != nil {
			log.WithField("process", "fetchBlueSkyAuthorFeed").
				WithField("profile", profile).
				Errorf("error mapping Bluesky post: %v", err)
			continue
		}
		if _, exists := seenPosts[outputPost.ID]; exists {
			log.WithField("process", "fetchBlueSkyAuthorFeed").
				WithField("profile", profile).
				WithField("item", outputPost.ID).
				Debug("Post already on database")
			continue
		}
		seenPosts[outputPost.ID] = struct{}{}
		items = append(items, outputPost)
	}

	return items, nil
}

func (r Reader) doBlueSkyRequest(requestURL string) (*http.Response, error) {
	resp, err := retryhttp.Do(context.Background(), r.httpClient(), retryhttp.Policy{MaxRetries: r.blueSkyRetryCount(), Delay: r.BlueSkyRetryDelay}, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("bluesky request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected Bluesky status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func mapBlueSkyAPIPost(post blueSkyAPIPost, requestedProfile string) (OutputPost, error) {
	if post.URI == "" {
		return OutputPost{}, fmt.Errorf("missing uri")
	}

	handle := post.Author.Handle
	if handle == "" {
		handle = requestedProfile
	}
	postURL := blueSkyPostURL(handle, post.URI)
	if postURL == "" {
		return OutputPost{}, fmt.Errorf("invalid post uri %q", post.URI)
	}

	publishedOn, err := parseBlueSkyPostTime(post.Record.CreatedAt, post.IndexedAt)
	if err != nil {
		return OutputPost{}, err
	}

	profileName := post.Author.DisplayName
	if profileName == "" {
		profileName = handle
	}

	return OutputPost{
		ID:          postURL,
		SType:       BlueSky,
		PublishedOn: publishedOn,
		BlueSkyPost: BlueSkyPost{
			BskyURI:        post.URI,
			BskyCID:        post.CID,
			Description:    blueSkyDescription(post),
			BskyProfileURI: blueSkyProfileURL(handle),
			BskyProfile:    profileName,
			BskyPost:       postURL,
		},
	}, nil
}

func blueSkyDescription(post blueSkyAPIPost) string {
	if post.Record.Text != "" {
		return post.Record.Text
	}
	for _, image := range post.Embed.Images {
		if image.Alt != "" {
			return image.Alt
		}
	}
	return ""
}

func parseBlueSkyPostTime(values ...string) (time.Time, error) {
	for _, value := range values {
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("missing valid Bluesky post time")
}

func blueSkyPostURL(handle, uri string) string {
	rkey := uri[strings.LastIndex(uri, "/")+1:]
	if rkey == "" || rkey == uri {
		return ""
	}
	return fmt.Sprintf("%s/post/%s", blueSkyProfileURL(handle), rkey)
}

func blueSkyProfileURL(handle string) string {
	return fmt.Sprintf("https://bsky.app/profile/%s", handle)
}

func (r Reader) fetchBlueSkyRSSFeed(profile string, seenPosts map[string]struct{}) ([]OutputPost, error) {
	feedURL := fmt.Sprintf("%s/rss", blueSkyProfileURL(profile))
	rssContent, err := r.getItems(feedURL)
	if err != nil {
		return nil, err
	}

	var allItems []OutputPost
	for _, item := range rssContent.Items {
		if _, exists := seenPosts[item.Link]; exists {
			log.WithField("process", "fetchBlueSkyRSSFeed").
				WithField("url", feedURL).
				WithField("item", item.Link).
				Debug("Post already on database")
			continue
		}

		oEmbed, err := r.fetchOEmbed(item.Link)
		if err != nil {
			log.WithField("process", "fetchBlueSkyRSSFeed").
				WithField("url", feedURL).
				WithField("item", item.Link).
				Errorf("Error fetching oEmbed: %v", err)
			continue
		}

		cid, err := r.extractBlueskyCID(oEmbed.HTML)
		if err != nil {
			log.WithField("process", "fetchBlueSkyRSSFeed").
				WithField("url", feedURL).
				WithField("item", item.Link).
				Errorf("Error extracting Bluesky CID: %v", err)
			continue
		}

		publishedTime := time.Time{}
		if item.PublishedParsed != nil {
			publishedTime = item.PublishedParsed.UTC()
		}
		if item.UpdatedParsed != nil && item.UpdatedParsed.After(publishedTime) {
			publishedTime = item.UpdatedParsed.UTC()
		}

		outputPost := OutputPost{
			ID:          item.Link,
			SType:       BlueSky,
			PublishedOn: publishedTime,
			BlueSkyPost: BlueSkyPost{
				BskyURI:        item.GUID,
				BskyCID:        cid,
				Description:    item.Description,
				BskyProfileURI: rssContent.Link,
				BskyProfile:    rssContent.Title,
				BskyPost:       item.Link,
			},
		}

		seenPosts[outputPost.ID] = struct{}{}
		allItems = append(allItems, outputPost)
	}

	return allItems, nil
}

func (r Reader) getItems(feedURL string) (*gofeed.Feed, error) {
	resp, err := retryhttp.Do(context.Background(), r.httpClient(), retryhttp.Policy{MaxRetries: r.blueSkyRetryCount(), Delay: r.BlueSkyRetryDelay}, func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected RSS status %d", resp.StatusCode)
	}

	return gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxResponseBytes))
}

func (r Reader) fetchOEmbed(link string) (oEmbedResponse, error) {
	requestURL := fmt.Sprintf("https://embed.bsky.app/oembed?url=%s", link)
	resp, err := retryhttp.Do(context.Background(), r.httpClient(), retryhttp.Policy{MaxRetries: r.blueSkyRetryCount(), Delay: r.BlueSkyRetryDelay}, func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	})
	if err != nil {
		return oEmbedResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return oEmbedResponse{}, fmt.Errorf("unexpected oEmbed status %d", resp.StatusCode)
	}

	var oembedResp oEmbedResponse
	err = json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&oembedResp)
	if err != nil {
		return oEmbedResponse{}, err
	}

	return oembedResp, nil
}

func (r Reader) httpClient() httpDoer {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return defaultBlueSkyHTTPClient
}

func (r Reader) blueSkyAuthorFeedEndpoint() string {
	if r.BlueSkyAuthorFeedEndpoint != "" {
		return r.BlueSkyAuthorFeedEndpoint
	}
	return defaultBlueSkyAuthorFeedEndpoint
}

func (r Reader) blueSkyRetryCount() int {
	if r.BlueSkyRetryCount > 0 {
		return r.BlueSkyRetryCount
	}
	return defaultBlueSkyRetryCount
}

func (r Reader) extractBlueskyCID(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("error parsing HTML: %w", err)
	}

	var f func(*html.Node) string
	f = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "blockquote" {
			for _, attr := range n.Attr {
				if attr.Key == "data-bluesky-cid" {
					return attr.Val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if cid := f(c); cid != "" {
				return cid
			}
		}
		return ""
	}

	cid := f(doc)
	if cid == "" {
		return "", fmt.Errorf("data-bluesky-cid attribute not found")
	}
	return cid, nil
}
