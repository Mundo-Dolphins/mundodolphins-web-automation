package cli

import (
	"context"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autopr"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/github"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/gitops"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/podcast"
)

const (
	rssCommitMessage = "Added new podcast episodes"
	rssPRTitle       = "Added new podcast episodes"
	rssPRBody        = "Automated pull request after adding new podcast episodes"
)

func newRSSAutoPRService(cfg autoconfig.Config, feedURL string, parser podcast.FeedParser) (autopr.Service, error) {
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
			episodes, err := parser.Parse(feedURL)
			if err != nil {
				return err
			}
			log.WithField("episodes", len(episodes)).Info("rss episodes fetched")
			return processRSSEpisodes(episodes, dataFolder, "", true)
		},
		Pathspec:      pathspec,
		CommitMessage: rssCommitMessage,
		PRTitle:       rssPRTitle,
		PRBody:        rssPRBody,
		PRLabel:       autoconfig.DefaultPRLabel,
	}, nil
}
