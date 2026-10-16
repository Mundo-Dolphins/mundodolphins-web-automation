package cli

import (
	"context"
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autopr"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/github"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/gitops"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/youtube"
)

const (
	youtubeCommitMessage = "Added new youtube videos"
	youtubePRTitle       = "Added new youtube videos"
	youtubePRBody        = "Automated pull request after adding new youtube videos and linking them to the podcast episodes"
)

// NewYouTubeCmd reads the YouTube channel feed into the data folder.
func NewYouTubeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "youtube",
		Short:  "Read the YouTube channel videos",
		Long:   "Reads the videos of the YouTube channel (the Data API with YOUTUBE_API_KEY and YOUTUBE_CHANNEL_ID, else the channel feed), adds the new ones to videos.json and links them to the podcast episodes",
		PreRun: toggleDebug,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, yt, err := autoconfig.FromYouTubeEnv()
			if err != nil {
				return err
			}
			source, err := youtubeSource(yt)
			if err != nil {
				return err
			}
			videos, err := source.Videos()
			if err != nil {
				return fmt.Errorf("reading YouTube videos: %w", err)
			}
			dataFolder, _ := cmd.Flags().GetString("data")
			res, err := processYouTubeVideos(videos, dataFolder)
			if err != nil {
				return fmt.Errorf("processing videos: %w", err)
			}
			log.WithFields(log.Fields{"fetched": len(videos), "added": res.Added, "updated": res.Updated, "linked": res.Linked}).Info("youtube videos synced")
			return nil
		},
	}
}

// NewCheckerYouTubeCmd reads the channel feed against the website repository
// and manages the pull request with the new videos.
func NewCheckerYouTubeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "youtube",
		Short:  "Read the YouTube channel feed and manage the videos pull request",
		Long:   "Read the configured YouTube channel feed against the configured website repository, commit data changes on a fixed branch, push it, and ensure a GitHub pull request exists.",
		PreRun: toggleDebug,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckerYouTube(cmd.Context())
		},
	}
}

func runCheckerYouTube(ctx context.Context) error {
	cfg, yt, err := autoconfig.FromYouTubeEnv()
	if err != nil {
		return err
	}
	source, err := youtubeSource(yt)
	if err != nil {
		return err
	}
	service, err := newYouTubeAutoPRService(cfg, source)
	if err != nil {
		return err
	}
	result, err := service.Run(ctx)
	if err != nil {
		return err
	}
	if !result.Locked {
		log.Info("another checker youtube execution is already running")
		return nil
	}
	if !result.Committed {
		log.Info("no video changes detected; pull request was not changed")
		return nil
	}
	log.WithField("pr", result.PullRequest.HTMLURL).Info("pull request ready")
	return nil
}

func newYouTubeAutoPRService(cfg autoconfig.Config, source youtube.Source) (autopr.Service, error) {
	dataFolder := filepath.Join(cfg.RepoPath, "data")
	pathspec, err := filepath.Rel(cfg.RepoPath, dataFolder)
	if err != nil {
		return autopr.Service{}, err
	}

	gitClient := gitops.New(cfg.RepoPath, cfg.GitTimeout, nil)
	ghClient := github.New(cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubToken, nil)
	return autopr.Service{
		Config:   cfg,
		LockPath: cfg.LockPath,
		Git:      gitClient,
		GitHub:   githubAdapter{client: ghClient},
		Generate: func(ctx context.Context) error {
			videos, err := source.Videos()
			if err != nil {
				return err
			}
			log.WithField("videos", len(videos)).Info("youtube videos fetched")
			res, err := processYouTubeVideos(videos, dataFolder)
			log.WithFields(log.Fields{"added": res.Added, "updated": res.Updated, "linked": res.Linked}).Info("youtube videos synced")
			return err
		},
		Pathspec:      pathspec,
		CommitMessage: youtubeCommitMessage,
		PRTitle:       youtubePRTitle,
		PRBody:        youtubePRBody,
		PRLabel:       autoconfig.DefaultPRLabel,
	}, nil
}
