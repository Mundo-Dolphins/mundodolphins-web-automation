package cli

import (
	"fmt"
	"os"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"

	"github.com/spf13/cobra"
)

// NewRssCmd allows injecting a parser (for testability)
func NewRssCmd(parser podcast.FeedParser) *cobra.Command {
	return &cobra.Command{
		Use:    "rss",
		Short:  "Scrape the RSS feed",
		Long:   "It scrapes the RSS feed for Mundo Dolphins Podcast",
		PreRun: toggleDebug,
		Run: func(cmd *cobra.Command, args []string) {
			feedURL, err := rssFeedURL(os.Getenv("RSS_FEED_URL"))
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			audios, err := parser.Parse(feedURL)
			if err != nil {
				fmt.Println("Error parsing RSS:", err)
				return
			}
			dataFolder, _ := cmd.Flags().GetString("data")
			since, _ := cmd.Flags().GetString("since")
			allSeasons, _ := cmd.Flags().GetBool("all-seasons")
			if err := processRSSEpisodes(audios, dataFolder, since, allSeasons); err != nil {
				fmt.Print(fmt.Errorf("error processing episodes: %v", err))
			}
		},
	}
}
