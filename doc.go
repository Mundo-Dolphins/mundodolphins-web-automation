// Package webautomation is the module root of mundodolphins-web-automation,
// which keeps the data folder of a static website in sync with a podcast RSS
// feed and with social posts, and can open a pull request with the result.
//
// Packages:
//
//   - podcast: episodes, RSS reader, season files and sync
//   - social: social posts model, Bluesky reader and the InstagramSource interface
//   - cli: the rss, social, social-autopr and checker commands
//   - autopr, gitops, github: branch, commit, push and pull request
//   - autoconfig, filelock, retryhttp: configuration, locking and HTTP retries
//
// The module does not scrape third-party websites: podcast.PageScraper and
// social.InstagramSource are extension points implemented by the caller.
// Until v1.0.0 the API may change between minor versions.
package webautomation
