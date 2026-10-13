package social

import (
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
)

type Type int

type OutputPost struct {
	ID            string        `json:"id"`
	SType         Type          `json:"stype"`
	PublishedOn   time.Time     `json:"PublishedOn"`
	BlueSkyPost   BlueSkyPost   `json:"BlueSkyPost"`
	InstagramPost InstagramPost `json:"InstagramPost"`
}

type InputPostId struct {
	ID    string
	SType Type
}

type InputFetch struct {
	IgProfile    string
	IgProfiles   []string
	BskyProfiles []string
	BskyFeeds    []string
	Posts        []InputPostId
}

const (
	BlueSky Type = iota
	Instagram
)

func (r Reader) FetchSocialPosts(input InputFetch) ([]OutputPost, error) {
	filter := func(i []InputPostId, t Type) []string {
		var f []string
		for _, postId := range i {
			if postId.SType == t {
				f = append(f, postId.ID)
			}
		}

		return f
	}

	log.Info("Fetching posts from BlueSky")
	p, err := r.fetchBlueSkyFeeds(input.BskyProfiles, input.BskyFeeds, filter(input.Posts, BlueSky))
	if err != nil {
		log.WithField("process", "FetchSocialPosts").Errorf("error fetching BlueSky posts: %v", err)
	}

	igProfiles := input.IgProfiles
	if len(igProfiles) == 0 && input.IgProfile != "" {
		igProfiles = []string{input.IgProfile}
	}

	var pi []OutputPost
	for _, profile := range igProfiles {
		log.WithField("profile", profile).Info("Fetching posts from Instagram")
		profilePosts, err := r.fetchInstagram(profile, filter(input.Posts, Instagram))
		if err != nil {
			log.WithField("process", "FetchSocialPosts").Errorf("error fetching Instagram posts: %v", err)
		}
		pi = append(pi, profilePosts...)
	}

	p = append(p, pi...)

	// Log for debugging order before sorting
	log.WithField("process", "FetchSocialPosts").
		Debugf("Before sorting: %d posts", len(p))
	for i, post := range p {
		log.WithField("process", "FetchSocialPosts").
			Debugf("Post %d: %s at %s (ID: %s)", i, post.ID, post.PublishedOn.Format("2006-01-02 15:04:05.000"), post.ID)
	}

	sort.SliceStable(p, func(i, j int) bool {
		// First sort by publish date
		if !p[i].PublishedOn.Equal(p[j].PublishedOn) {
			return p[i].PublishedOn.After(p[j].PublishedOn)
		}
		// If dates are equal, use the ID as a deterministic tiebreaker
		// For BlueSky posts the ID is the post link and is unique
		// For Instagram posts the ID should also be unique
		return p[i].ID < p[j].ID
	})

	// Log for debugging order after sorting
	log.WithField("process", "FetchSocialPosts").
		Debugf("After sorting: %d posts", len(p))
	for i, post := range p {
		log.WithField("process", "FetchSocialPosts").
			Debugf("Post %d: %s at %s (ID: %s)", i, post.ID, post.PublishedOn.Format("2006-01-02 15:04:05.000"), post.ID)
	}

	log.Infof("Found %d posts", len(p))
	return p, nil
}

func (r Reader) fetchInstagram(profile string, known []string) ([]OutputPost, error) {
	if r.Instagram == nil {
		log.WithField("profile", profile).Debug("no Instagram source configured; skipping")
		return nil, nil
	}
	return r.Instagram.FetchProfile(profile, known)
}
