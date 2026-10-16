// Package cli builds the command line of the web automation: it reads a podcast
// RSS feed and social posts (Bluesky, plus any InstagramSource the caller
// provides) into the data folder of a website repository, and can open a pull
// request with the result.
package cli

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/youtube"
)

// Options carries everything that depends on who runs the automation.
type Options struct {
	// Use, Short and Long describe the root command.
	Use, Short, Long string
	// FeedURL is the podcast RSS feed read when RSS_FEED_URL is not set.
	FeedURL string
	// FeedParser reads the podcast feed. Defaults to &podcast.RSSParser{}.
	FeedParser podcast.FeedParser
	// YouTubeChannelID is the channel read when YOUTUBE_CHANNEL_ID is not set.
	YouTubeChannelID string
	// YouTubeSource reads the channel videos. By default it is the YouTube Data
	// API when YOUTUBE_API_KEY is set, else the channel Atom feed.
	YouTubeSource youtube.Source
	// Social reads the social posts. Defaults to a social.Reader without
	// Instagram (see social.InstagramSource).
	Social social.ReaderInterface
	// InstagramProfiles and BlueSkyProfiles are the profiles read when the
	// SOCIAL_INSTAGRAM_PROFILES / SOCIAL_BSKY_PROFILES variables are not set.
	InstagramProfiles []string
	BlueSkyProfiles   []string
	// PreferAudio is passed to podcast.SyncOptions.
	PreferAudio func(url string) bool
}

// settings holds the options of the command tree built by New. A process
// builds one command tree, so a package variable keeps the constructors simple.
var settings Options

var debug bool

// New builds the root command with the rss, youtube, social, social-autopr and checker
// subcommands. Callers can add their own subcommands to the result.
func New(opts Options) *cobra.Command {
	if opts.Use == "" {
		opts.Use = "web-automation"
	}
	if opts.Short == "" {
		opts.Short = "Keeps a website data folder in sync with a podcast feed and social posts"
	}
	if opts.FeedParser == nil {
		opts.FeedParser = &podcast.RSSParser{}
	}
	if opts.Social == nil {
		opts.Social = social.Reader{}
	}
	settings = opts

	if opts.Long == "" {
		opts.Long = "Reads a podcast RSS feed and social posts (Bluesky, and Instagram through a caller-provided source) into a website data folder, optionally opening a pull request."
	}
	root := &cobra.Command{Use: opts.Use, Short: opts.Short, Long: opts.Long}
	root.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "verbose logging")

	rss := NewRssCmd(opts.FeedParser)
	rss.Flags().String("data", "./data", "Data folder")
	rss.Flags().String("since", "", "Process only episodes strictly newer than this RFC3339 timestamp")
	rss.Flags().Bool("all-seasons", false, "Process and rewrite all seasons instead of only the active season")
	social := NewSocialCmd(opts.Social)
	social.Flags().String("data", "./data", "Data folder")

	yt := NewYouTubeCmd()
	yt.Flags().String("data", "./data", "Data folder")

	checker := NewCheckerCmd(opts.FeedParser, opts.Social)
	checker.AddCommand(NewCheckerYouTubeCmd())

	root.AddCommand(rss, yt, social, NewSocialAutoPRCmd(opts.Social), checker)
	return root
}

// Execute configures logging and runs the command.
func Execute(root *cobra.Command) {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func toggleDebug(cmd *cobra.Command, args []string) {
	if debug {
		log.Info("Debug logs enabled")
		log.SetLevel(log.DebugLevel)
	}
}

// rssFeedURL is the feed to read: RSS_FEED_URL, else the configured default.
func rssFeedURL(fromEnv string) (string, error) {
	if fromEnv != "" {
		return fromEnv, nil
	}
	if settings.FeedURL != "" {
		return settings.FeedURL, nil
	}
	return "", fmt.Errorf("no podcast feed configured: set RSS_FEED_URL")
}

var processRSSEpisodes = func(episodes []podcast.Episode, dataFolder, since string, allSeasons bool) error {
	return podcast.Sync(episodes, dataFolder, podcast.SyncOptions{Since: since, AllSeasons: allSeasons, PreferAudio: settings.PreferAudio})
}

// youtubeSource is the source of the channel videos: the configured one, else
// the YouTube Data API when there is an API key, else the channel feed.
func youtubeSource(yt autoconfig.YouTubeSettings) (youtube.Source, error) {
	if settings.YouTubeSource != nil {
		return settings.YouTubeSource, nil
	}
	if yt.ChannelID == "" {
		yt.ChannelID = settings.YouTubeChannelID
	}
	if yt.APIKey != "" {
		if yt.ChannelID == "" {
			return nil, fmt.Errorf("YOUTUBE_API_KEY needs YOUTUBE_CHANNEL_ID")
		}
		return &youtube.APISource{APIKey: yt.APIKey, ChannelID: yt.ChannelID, MaxPages: yt.MaxPages}, nil
	}
	if yt.FeedURL == "" && yt.ChannelID != "" {
		yt.FeedURL = youtube.FeedURL(yt.ChannelID)
	}
	if yt.FeedURL == "" {
		return nil, fmt.Errorf("no YouTube source configured: set YOUTUBE_CHANNEL_ID (and YOUTUBE_API_KEY for duration and embeddable)")
	}
	return &youtube.RSSSource{FeedURL: yt.FeedURL}, nil
}

var processYouTubeVideos = func(videos []youtube.Video, dataFolder string) (youtube.Result, error) {
	return youtube.Sync(videos, dataFolder)
}
