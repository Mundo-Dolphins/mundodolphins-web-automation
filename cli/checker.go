package cli

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

func NewCheckerCmd(parser podcast.FeedParser, reader social.ReaderInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checker",
		Short: "Run autonomous repository checkers",
	}
	cmd.AddCommand(NewCheckerRSSCmd(parser))
	cmd.AddCommand(NewCheckerSocialCmd(reader))
	return cmd
}

func NewCheckerRSSCmd(parser podcast.FeedParser) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "rss",
		Short:  "Read the podcast RSS feed and manage the podcast episodes pull request",
		Long:   "Read the configured podcast RSS feed against the configured website repository, commit data changes on a fixed branch, push it, and ensure a GitHub pull request exists.",
		PreRun: toggleDebug,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckerRSS(cmd.Context(), parser)
		},
	}
	return cmd
}

func runCheckerRSS(ctx context.Context, parser podcast.FeedParser) error {
	cfg, feedURL, err := autoconfig.FromRSSEnv()
	if err != nil {
		return err
	}
	if feedURL, err = rssFeedURL(feedURL); err != nil {
		return err
	}

	schedule, err := autoconfig.FromRSSCheckScheduleEnv()
	if err != nil {
		return err
	}
	if !schedule.Allows(time.Now()) {
		log.WithFields(log.Fields{
			"windows":  schedule.Spec,
			"timezone": schedule.Location.String(),
		}).Info("rss checker skipped outside configured check windows")
		return nil
	}

	service, err := newRSSAutoPRService(cfg, feedURL, parser)
	if err != nil {
		return err
	}

	result, err := service.Run(ctx)
	if err != nil {
		return err
	}
	if !result.Locked {
		log.Info("another checker rss execution is already running")
		return nil
	}
	if !result.Committed {
		log.Info("no podcast data changes detected; pull request was not changed")
		return nil
	}
	log.WithField("pr", result.PullRequest.HTMLURL).Info("pull request ready")
	return nil
}

func NewCheckerSocialCmd(reader social.ReaderInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "social",
		Short:  "Scrape social feeds and manage the social posts pull request",
		Long:   "Scrape social feeds against the configured website repository, commit data changes on a fixed branch, push it, and ensure a GitHub pull request exists.",
		PreRun: toggleDebug,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckerSocial(cmd.Context(), reader)
		},
	}
	return cmd
}

func runCheckerSocial(ctx context.Context, reader social.ReaderInterface) error {
	cfg, err := autoconfig.FromSocialEnv()
	if err != nil {
		return err
	}

	service, err := newSocialAutoPRService(cfg, "", reader)
	if err != nil {
		return err
	}

	result, err := service.Run(ctx)
	if err != nil {
		return err
	}
	if !result.Locked {
		log.Info("another checker social execution is already running")
		return nil
	}
	if !result.Committed {
		log.Info("no data changes detected; pull request was not changed")
		return nil
	}
	log.WithField("pr", result.PullRequest.HTMLURL).Info("pull request ready")
	return nil
}
