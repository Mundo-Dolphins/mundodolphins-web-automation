package podcast

// FeedParser reads the episodes of a podcast RSS feed.
type FeedParser interface {
	Parse(url string) ([]Episode, error)
}

// PageScraper extracts episodes from a podcast web page. The library ships no
// implementation: scraping a third-party site is the caller's responsibility.
type PageScraper interface {
	Audios(url string) ([]Episode, error)
}
