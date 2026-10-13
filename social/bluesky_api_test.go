package social

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchBlueSkyPublicAPI_ProfileWithoutNewPosts(t *testing.T) {
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, blueSkyFeedResponse("alice.example.com", "3new", "cid1", "Already stored"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
	}

	existing := []string{"https://bsky.app/profile/alice.example.com/post/3new"}
	posts, err := reader.fetchBlueSkyFeeds([]string{"alice.example.com"}, nil, existing)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected no new posts, got %d", len(posts))
	}
}

func TestFetchBlueSkyPublicAPI_ProfileWithNewPosts(t *testing.T) {
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("actor"); got != "team.example.com" {
			t.Fatalf("expected actor query to be team.example.com, got %q", got)
		}
		if got := r.URL.Query().Get("filter"); got != "posts_no_replies" {
			t.Fatalf("expected filter query to be posts_no_replies, got %q", got)
		}
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3new", "cid1", "Fresh post"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
	}

	posts, err := reader.fetchBlueSkyFeeds([]string{"team.example.com"}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	post := posts[0]
	if post.ID != "https://bsky.app/profile/team.example.com/post/3new" {
		t.Fatalf("unexpected stable id: %s", post.ID)
	}
	if post.BlueSkyPost.BskyURI != "at://did:plc:test/app.bsky.feed.post/3new" {
		t.Fatalf("unexpected bsky uri: %s", post.BlueSkyPost.BskyURI)
	}
	if post.BlueSkyPost.BskyCID != "cid1" {
		t.Fatalf("unexpected cid: %s", post.BlueSkyPost.BskyCID)
	}
	if post.BlueSkyPost.Description != "Fresh post" {
		t.Fatalf("unexpected description: %s", post.BlueSkyPost.Description)
	}
}

func TestFetchBlueSkyPublicAPI_MultipleProfilesDeduplicateDuplicates(t *testing.T) {
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3same", "cid1", "Duplicate"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
	}

	posts, err := reader.fetchBlueSkyFeeds(
		[]string{"team.example.com", "bob.example.com"},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected duplicate posts to collapse to 1, got %d", len(posts))
	}
}

func TestFetchBlueSkyPublicAPI_Retries429And5xx(t *testing.T) {
	requests := 0
	statuses := []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusOK}
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		status := statuses[requests]
		requests++
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3new", "cid1", "Fresh post"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
		BlueSkyRetryCount:         2,
	}

	posts, err := reader.fetchBlueSkyFeeds([]string{"team.example.com"}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests, got %d", requests)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post after retry, got %d", len(posts))
	}
}

func TestFetchBlueSkyPublicAPI_InvalidJSONDoesNotBlockOtherProfiles(t *testing.T) {
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("actor") == "broken.example.com" {
			fmt.Fprint(w, "{invalid")
			return
		}
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3new", "cid1", "Fresh post"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
	}

	posts, err := reader.fetchBlueSkyFeeds(
		[]string{"broken.example.com", "team.example.com"},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("expected partial failure to be non-fatal, got %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected valid profile to be processed, got %d posts", len(posts))
	}
}

type stubInstagram struct {
	posts []OutputPost
	err   error
	calls int
	known []string
}

func (s *stubInstagram) FetchProfile(_ string, known []string) ([]OutputPost, error) {
	s.calls++
	s.known = known
	return s.posts, s.err
}

func TestFetchSocialPostsInstagramFailureDoesNotBlockBlueSky(t *testing.T) {
	instagram := &stubInstagram{err: errors.New("instagram unavailable")}

	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3new", "cid1", "Fresh post"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                server.Client(),
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
		Instagram:                 instagram,
	}

	posts, err := reader.FetchSocialPosts(InputFetch{
		IgProfiles:   []string{"some_profile"},
		BskyProfiles: []string{"team.example.com"},
	})
	if err != nil {
		t.Fatalf("FetchSocialPosts() error = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(posts))
	}
	if posts[0].SType != BlueSky {
		t.Fatalf("post type = %v, want BlueSky", posts[0].SType)
	}
	if instagram.calls != 1 {
		t.Fatalf("the Instagram source should have been asked once, got %d", instagram.calls)
	}
}

func TestFetchSocialPostsUsesInstagramSourceAndSkipsItWhenAbsent(t *testing.T) {
	post := OutputPost{ID: "https://www.instagram.com/p/ABC/", SType: Instagram, PublishedOn: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	instagram := &stubInstagram{posts: []OutputPost{post}}

	posts, err := Reader{Instagram: instagram}.FetchSocialPosts(InputFetch{
		IgProfiles: []string{"p1"},
		Posts:      []InputPostId{{ID: "https://www.instagram.com/p/OLD/", SType: Instagram}, {ID: "bsky-post", SType: BlueSky}},
	})
	if err != nil || len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("expected the Instagram post, got %+v err=%v", posts, err)
	}
	if len(instagram.known) != 1 || instagram.known[0] != "https://www.instagram.com/p/OLD/" {
		t.Fatalf("only Instagram IDs must be passed as known, got %v", instagram.known)
	}

	posts, err = Reader{}.FetchSocialPosts(InputFetch{IgProfiles: []string{"p1"}})
	if err != nil || len(posts) != 0 {
		t.Fatalf("without a source Instagram must be skipped silently, got %+v err=%v", posts, err)
	}
}

func TestFetchBlueSkyPublicAPITimeoutControlled(t *testing.T) {
	server := newBlueSkyAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, blueSkyFeedResponse("team.example.com", "3new", "cid1", "Fresh post"))
	})
	defer server.Close()

	reader := Reader{
		HTTPClient:                &http.Client{Timeout: time.Millisecond},
		BlueSkyAuthorFeedEndpoint: server.URL + "/xrpc/app.bsky.feed.getAuthorFeed",
		BlueSkyRetryCount:         1,
	}

	_, err := reader.fetchBlueSkyFeeds([]string{"team.example.com"}, nil, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func newBlueSkyAPITestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/app.bsky.feed.getAuthorFeed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		handler(w, r)
	}))
}

func blueSkyFeedResponse(handle, rkey, cid, text string) string {
	return fmt.Sprintf(`{
		"feed": [{
			"post": {
				"uri": "at://did:plc:test/app.bsky.feed.post/%s",
				"cid": %q,
				"author": {
					"handle": %q,
					"displayName": "Mundo Dolphins"
				},
				"record": {
					"text": %q,
					"createdAt": "2026-09-09T10:00:00.000Z"
				},
				"indexedAt": "2026-09-09T10:01:00.000Z"
			}
		}]
	}`, rkey, cid, handle, text)
}
