package cli

import (
	"context"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autoconfig"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/autopr"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/github"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/gitops"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

const (
	socialCommitMessage = "Added new social posts"
	socialPRTitle       = "Added new social posts"
	socialPRBody        = "Automated pull request after adding new social posts"
)

type githubAdapter struct {
	client *github.Client
}

func (a githubAdapter) EnsurePullRequest(ctx context.Context, title string, body string, head string, base string, label string) (autopr.PullRequest, bool, error) {
	pr, created, err := a.client.EnsurePullRequest(ctx, title, body, head, base, label)
	if err != nil {
		if github.IsNoCommitsBetween(err) {
			return autopr.PullRequest{}, false, autopr.ErrNoBranchChanges
		}
		return autopr.PullRequest{}, false, err
	}
	return autopr.PullRequest{Number: pr.Number, HTMLURL: pr.HTMLURL}, created, nil
}

func NewSocialAutoPRCmd(reader social.ReaderInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "social-autopr",
		Short:  "Scrape social feeds and open or update an automated pull request",
		Long:   "Scrape social feeds, commit changes on a fixed work branch, push it, and ensure a GitHub pull request exists.",
		PreRun: toggleDebug,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg, err := autoconfig.FromSocialEnv()
			if err != nil {
				log.Errorf("error loading config: %v", err)
				return
			}

			dataFolder, _ := cmd.Flags().GetString("data")
			service, err := newSocialAutoPRService(cfg, dataFolder, reader)
			if err != nil {
				log.Errorf("error creating social autopr service: %v", err)
				return
			}

			result, err := service.Run(ctx)
			if err != nil {
				log.Errorf("error running social autopr: %v", err)
				return
			}
			if !result.Locked {
				log.Info("another social autopr execution is already running")
				return
			}
			if !result.Committed {
				log.Info("no staged changes detected; pull request was not changed")
				return
			}
			log.WithField("pr", result.PullRequest.HTMLURL).Info("pull request ready")
		},
	}
	cmd.Flags().String("data", "", "Data folder")
	return cmd
}

func repoPathFromEnv() string {
	if repoPath := os.Getenv("SOCIAL_REPOSITORY_PATH"); repoPath != "" {
		return repoPath
	}
	if repoPath := os.Getenv("REPO_PATH"); repoPath != "" {
		return repoPath
	}
	return os.Getenv("GITHUB_REPO_PATH")
}

func newSocialAutoPRService(cfg autoconfig.Config, dataFolder string, reader social.ReaderInterface) (autopr.Service, error) {
	if dataFolder == "" {
		dataFolder = filepath.Join(cfg.RepoPath, "data")
	}
	if repoPathFromEnv() == "" {
		cfg.RepoPath = filepath.Clean(filepath.Dir(dataFolder))
	}

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
			return runSocialWithOptions(ctx, reader, dataFolder, socialRunOptionsFromEnv())
		},
		Pathspec:      pathspec,
		CommitMessage: socialCommitMessage,
		PRTitle:       socialPRTitle,
		PRBody:        socialPRBody,
		PRLabel:       autoconfig.DefaultPRLabel,
	}, nil
}
