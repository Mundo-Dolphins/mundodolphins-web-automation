package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

const postsPerFile = 50

type socialRunOptions struct {
	InstagramProfiles []string
	BlueSkyProfiles   []string
}

// NewSocialCmd permite inyectar un reader (para testabilidad)
func NewSocialCmd(reader social.ReaderInterface) *cobra.Command {
	return &cobra.Command{
		Use:    "social",
		Short:  "Scrape the social feeds",
		Long:   "It scrapes the social feeds for Mundo Dolphins",
		PreRun: toggleDebug,
		Run: func(cmd *cobra.Command, args []string) {
			dataFolder, _ := cmd.Flags().GetString("data")

			if err := runSocial(context.Background(), reader, dataFolder); err != nil {
				log.Errorf("error running social scraper: %v", err)
			}
		},
	}
}

func runSocial(ctx context.Context, reader social.ReaderInterface, dataFolder string) error {
	return runSocialWithOptions(ctx, reader, dataFolder, defaultSocialRunOptions())
}

func runSocialWithOptions(ctx context.Context, reader social.ReaderInterface, dataFolder string, options socialRunOptions) error {
	_ = ctx
	allPosts, err := loadPostsURIsFromJSON(dataFolder)
	if err != nil {
		return err
	}

	var postIDs []social.InputPostId
	for _, post := range allPosts {
		// Instagram entries without a date are leftovers of the old
		// instagram.com scraping (link only); let the feed fill them again.
		if incompleteInstagramPost(post) {
			continue
		}
		postIDs = append(postIDs, social.InputPostId{
			ID:    post.ID,
			SType: post.SType,
		})
	}

	posts, err := reader.FetchSocialPosts(
		social.InputFetch{
			IgProfiles:   options.InstagramProfiles,
			BskyProfiles: options.BlueSkyProfiles,
			Posts:        postIDs,
		},
	)
	if err != nil {
		return err
	}
	log.Debugf("Found %d posts", len(posts))
	if len(posts) == 0 {
		log.Info("no new social posts found")
		return nil
	}
	log.WithField("bluesky", countPostsByType(posts, social.BlueSky)).
		WithField("instagram", countPostsByType(posts, social.Instagram)).
		Info("new social posts found")

	posts = append(posts, withoutReplacedInstagramPosts(allPosts, posts)...)
	posts = uniqueElements(posts)
	log.Debugf("Found %d unique posts", len(posts))

	sort.SliceStable(posts, func(i, j int) bool {
		if !posts[i].PublishedOn.Equal(posts[j].PublishedOn) {
			return posts[i].PublishedOn.After(posts[j].PublishedOn)
		}
		return posts[i].ID < posts[j].ID
	})

	return savePostsToJSON(posts, dataFolder)
}

func incompleteInstagramPost(post social.OutputPost) bool {
	return post.SType == social.Instagram && post.PublishedOn.IsZero()
}

// withoutReplacedInstagramPosts drops the existing Instagram entries that a
// fetched post replaces, matching /p/ and /reel/ URLs of the same post.
func withoutReplacedInstagramPosts(existing, fetched []social.OutputPost) []social.OutputPost {
	replaced := map[string]bool{}
	for _, post := range fetched {
		if post.SType != social.Instagram {
			continue
		}
		replaced[post.ID] = true
		if code := social.InstagramShortcode(post.ID); code != "" {
			replaced[code] = true
		}
	}
	kept := make([]social.OutputPost, 0, len(existing))
	for _, post := range existing {
		if post.SType == social.Instagram && (replaced[post.ID] || replaced[social.InstagramShortcode(post.ID)]) {
			continue
		}
		kept = append(kept, post)
	}
	return kept
}

func countPostsByType(posts []social.OutputPost, postType social.Type) int {
	count := 0
	for _, post := range posts {
		if post.SType == postType {
			count++
		}
	}
	return count
}

func defaultSocialRunOptions() socialRunOptions {
	return socialRunOptions{
		InstagramProfiles: settings.InstagramProfiles,
		BlueSkyProfiles:   settings.BlueSkyProfiles,
	}
}

func socialRunOptionsFromEnv() socialRunOptions {
	options := defaultSocialRunOptions()
	if profiles := splitEnvList(os.Getenv("SOCIAL_INSTAGRAM_PROFILES")); len(profiles) > 0 {
		options.InstagramProfiles = profiles
	}
	if profiles := splitEnvList(os.Getenv("SOCIAL_BSKY_PROFILES")); len(profiles) > 0 {
		options.BlueSkyProfiles = profiles
	}
	return options
}

func splitEnvList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\t'
	})
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

var savePostsToJSON = func(posts []social.OutputPost, dir string) error {
	log.WithField("count", len(posts)).Debug("saving posts")
	for i := 0; i < len(posts); i += postsPerFile {
		end := i + postsPerFile
		if end > len(posts) {
			end = len(posts)
		}
		chunk := posts[i:end]
		fileName := filepath.Join(dir, fmt.Sprintf("posts_%d.json", i/postsPerFile+1))
		if err := saveChunkToFile(chunk, fileName); err != nil {
			return err
		}
	}
	return nil
}

func saveChunkToFile(posts []social.OutputPost, fileName string) error {
	log.WithField("path", fileName).Debugf("Saving %d posts", len(posts))
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.WithField("path", fileName).Errorf("Error closing file: %v", err)

		}
	}(file)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(posts); err != nil {
		return err
	}
	log.WithField("path", fileName).Debugf("Saved %d posts", len(posts))
	return nil
}

var loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
	var postsURIs []social.OutputPost

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "posts_") && strings.HasSuffix(info.Name(), ".json") {
			log.WithField("path", path).Debug("loading posts")
			fileContent, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var post []social.OutputPost
			err = json.Unmarshal(fileContent, &post)
			if err != nil {
				return err
			}

			log.WithField("path", path).Debugf("loaded %d posts", len(post))
			postsURIs = append(postsURIs, post...)
		}
		return nil
	})
	if err != nil {
		log.Errorf("Error walking through files: %v\n", err)
		return nil, err
	}

	return postsURIs, nil
}

func uniqueElements(posts []social.OutputPost) []social.OutputPost {
	uniqueMap := make(map[string]social.OutputPost)
	for _, post := range posts {
		uniqueMap[post.ID] = post
	}

	var uniquePosts []social.OutputPost
	for _, post := range uniqueMap {
		uniquePosts = append(uniquePosts, post)
	}

	return uniquePosts
}
