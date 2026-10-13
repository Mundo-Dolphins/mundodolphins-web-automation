package social

import "regexp"

// InstagramPost is the Instagram part of an OutputPost. The JSON layout is
// consumed by the website, so it must not change.
type InstagramPost struct {
	URL         string `json:"URL"`
	Description string `json:"Description"`
	SvgPath     string `json:"SvgPath"`
}

// InstagramSource reads the posts of an Instagram profile.
//
// The library ships no implementation: reading Instagram means scraping it or
// relying on a service that does, and whether that is allowed is up to the
// caller. known holds the IDs (post URLs) already stored; implementations
// should skip those posts. Return posts with SType set to Instagram.
type InstagramSource interface {
	FetchProfile(profile string, known []string) ([]OutputPost, error)
}

var instagramShortcodeRe = regexp.MustCompile(`instagram\.com/(?:[^/?#]+/)?(?:p|reel|tv)/([^/?#]+)`)

// InstagramShortcode returns the post code of an Instagram post URL, which is
// shared by its /p/ and /reel/ forms, or "" when the URL is not a post.
func InstagramShortcode(postURL string) string {
	if m := instagramShortcodeRe.FindStringSubmatch(postURL); m != nil {
		return m[1]
	}
	return ""
}
