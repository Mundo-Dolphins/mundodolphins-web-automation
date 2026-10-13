package cli

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

type mockReader struct {
	social.ReaderInterface
	FetchFunc func(input social.InputFetch) ([]social.OutputPost, error)
}

func (m *mockReader) FetchSocialPosts(input social.InputFetch) ([]social.OutputPost, error) {
	return m.FetchFunc(input)
}

func TestSocialCmd_Success(t *testing.T) {
	called := false
	mock := &mockReader{
		FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
			called = true
			return []social.OutputPost{{ID: "id1", PublishedOn: time.Now()}}, nil
		},
	}
	cmd := NewSocialCmd(mock)
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.SetArgs([]string{"--data", "./testdata"})
	oldLoad := loadPostsURIsFromJSON
	oldSave := savePostsToJSON
	defer func() {
		loadPostsURIsFromJSON = oldLoad
		savePostsToJSON = oldSave
	}()
	loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
		return []social.OutputPost{{ID: "id2", PublishedOn: time.Now()}}, nil
	}
	savePostsToJSON = func(posts []social.OutputPost, dir string) error {
		if len(posts) != 2 {
			t.Errorf("expected 2 posts, got %+v", posts)
		}
		return nil
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando comando: %v", err)
	}
	if !called {
		t.Error("mock FetchSocialPosts was not called")
	}
}

func TestSocialCmd_Error(t *testing.T) {
	mock := &mockReader{
		FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
			return nil, errors.New("fallo de red")
		},
	}
	cmd := NewSocialCmd(mock)
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.SetArgs([]string{"--data", "./testdata"})
	oldLoad := loadPostsURIsFromJSON
	oldSave := savePostsToJSON
	defer func() {
		loadPostsURIsFromJSON = oldLoad
		savePostsToJSON = oldSave
	}()
	loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
		return []social.OutputPost{{ID: "id2", PublishedOn: time.Now()}}, nil
	}
	savePostsToJSON = func(posts []social.OutputPost, dir string) error {
		t.Error("savePostsToJSON should not be called on error")
		return nil
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando comando: %v", err)
	}
}

func TestRunSocial_NoNewPostsDoesNotSave(t *testing.T) {
	oldSettings := settings
	settings.BlueSkyProfiles = []string{"default.example.com"}
	defer func() { settings = oldSettings }()
	mock := &mockReader{
		FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
			if !slices.Equal(input.BskyProfiles, []string{"default.example.com"}) {
				t.Fatalf("expected default Bluesky profiles, got %+v", input.BskyProfiles)
			}
			return []social.OutputPost{}, nil
		},
	}

	oldLoad := loadPostsURIsFromJSON
	oldSave := savePostsToJSON
	defer func() {
		loadPostsURIsFromJSON = oldLoad
		savePostsToJSON = oldSave
	}()

	loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
		return []social.OutputPost{
			{
				ID:          "https://bsky.app/profile/team.example.com/post/existing",
				SType:       social.BlueSky,
				PublishedOn: time.Now(),
			},
		}, nil
	}
	savePostsToJSON = func(posts []social.OutputPost, dir string) error {
		t.Fatal("savePostsToJSON should not be called without new posts")
		return nil
	}

	if err := runSocial(t.Context(), mock, "./testdata"); err != nil {
		t.Fatalf("runSocial failed: %v", err)
	}
}

func TestRunSocialWithOptionsUsesConfiguredProfiles(t *testing.T) {
	mock := &mockReader{
		FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
			if !slices.Equal(input.BskyProfiles, []string{"one.bsky", "two.bsky"}) {
				t.Fatalf("BskyProfiles = %+v", input.BskyProfiles)
			}
			if !slices.Equal(input.IgProfiles, []string{"ig_one", "ig_two"}) {
				t.Fatalf("IgProfiles = %+v", input.IgProfiles)
			}
			return []social.OutputPost{}, nil
		},
	}

	oldLoad := loadPostsURIsFromJSON
	oldSave := savePostsToJSON
	defer func() {
		loadPostsURIsFromJSON = oldLoad
		savePostsToJSON = oldSave
	}()

	loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
		return nil, nil
	}
	savePostsToJSON = func(posts []social.OutputPost, dir string) error {
		t.Fatal("savePostsToJSON should not be called without new posts")
		return nil
	}

	err := runSocialWithOptions(t.Context(), mock, "./testdata", socialRunOptions{
		InstagramProfiles: []string{"ig_one", "ig_two"},
		BlueSkyProfiles:   []string{"one.bsky", "two.bsky"},
	})
	if err != nil {
		t.Fatalf("runSocialWithOptions failed: %v", err)
	}
}

func TestSocialCmd_OrderStability(t *testing.T) {
	// Crear posts con la misma fecha para probar la estabilidad del orden
	sameTime := time.Date(2025, 10, 6, 19, 13, 0, 0, time.UTC)

	// Posts that could swap order if there is no deterministic criterion
	testPosts := []social.OutputPost{
		{
			ID:          "https://bsky.app/profile/bob.example.com/post/3m2kd22ttdc2x",
			SType:       social.BlueSky,
			PublishedOn: sameTime,
		},
		{
			ID:          "https://bsky.app/profile/bob.example.com/post/3m2kcywcoak2x",
			SType:       social.BlueSky,
			PublishedOn: sameTime,
		},
	}

	var savedPosts []social.OutputPost

	mock := &mockReader{
		FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
			return testPosts, nil
		},
	}

	cmd := NewSocialCmd(mock)
	cmd.Flags().String("data", "./testdata", "Data folder")
	cmd.SetArgs([]string{"--data", "./testdata"})

	oldLoad := loadPostsURIsFromJSON
	oldSave := savePostsToJSON
	defer func() {
		loadPostsURIsFromJSON = oldLoad
		savePostsToJSON = oldSave
	}()

	loadPostsURIsFromJSON = func(dir string) ([]social.OutputPost, error) {
		return []social.OutputPost{}, nil // No existing posts
	}

	savePostsToJSON = func(posts []social.OutputPost, dir string) error {
		savedPosts = make([]social.OutputPost, len(posts))
		copy(savedPosts, posts)
		return nil
	}

	// Run the command multiple times to verify consistency
	for i := 0; i < 5; i++ {
		savedPosts = nil

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Iteration %d: error executing command: %v", i, err)
		}

		if len(savedPosts) != 2 {
			t.Fatalf("Iteration %d: expected 2 posts, got %d", i, len(savedPosts))
		}

		// Verify that the order is consistent
		// The post with ID "3m2kcywcoak2x" should come before "3m2kd22ttdc2x"
		// because "3m2kcywcoak2x" < "3m2kd22ttdc2x" lexicographically
		expectedFirstID := "https://bsky.app/profile/bob.example.com/post/3m2kcywcoak2x"

		if savedPosts[0].ID != expectedFirstID {
			t.Errorf("Iteration %d: Inconsistent order. Expected first ID: %s, got: %s",
				i, expectedFirstID, savedPosts[0].ID)
		}
	}
}

func TestRunSocialRepairsIncompleteInstagramEntries(t *testing.T) {
	published := time.Date(2026, 9, 20, 7, 28, 47, 0, time.UTC)
	complete := social.OutputPost{ID: "https://www.instagram.com/p/OK/", SType: social.Instagram, PublishedOn: published}
	existing := []social.OutputPost{
		complete,
		// Broken entries from the old instagram.com scraping: link only.
		{ID: "https://www.instagram.com/reel/BROKEN/", SType: social.Instagram},
		{ID: "https://www.instagram.com/p/STALE/", SType: social.Instagram},
	}

	oldLoad, oldSave := loadPostsURIsFromJSON, savePostsToJSON
	t.Cleanup(func() { loadPostsURIsFromJSON, savePostsToJSON = oldLoad, oldSave })
	loadPostsURIsFromJSON = func(string) ([]social.OutputPost, error) { return existing, nil }
	var saved []social.OutputPost
	savePostsToJSON = func(posts []social.OutputPost, _ string) error { saved = posts; return nil }

	var known []string
	repaired := social.OutputPost{
		ID:            "https://www.instagram.com/p/BROKEN/",
		SType:         social.Instagram,
		PublishedOn:   published.Add(time.Hour),
		InstagramPost: social.InstagramPost{URL: "https://www.instagram.com/p/BROKEN/", Description: "caption"},
	}
	reader := &mockReader{FetchFunc: func(input social.InputFetch) ([]social.OutputPost, error) {
		for _, p := range input.Posts {
			known = append(known, p.ID)
		}
		return []social.OutputPost{repaired}, nil
	}}

	if err := runSocialWithOptions(t.Context(), reader, "data", defaultSocialRunOptions()); err != nil {
		t.Fatalf("runSocialWithOptions: %v", err)
	}
	if !slices.Equal(known, []string{complete.ID}) {
		t.Fatalf("incomplete Instagram entries must not count as known, got %v", known)
	}

	var ids []string
	for _, p := range saved {
		ids = append(ids, p.ID)
	}
	want := []string{repaired.ID, complete.ID, "https://www.instagram.com/p/STALE/"}
	if !slices.Equal(ids, want) {
		t.Fatalf("expected the repaired post to replace the broken reel entry, got %v", ids)
	}
	if saved[0].InstagramPost.Description != "caption" {
		t.Fatalf("the repaired entry must keep the new data, got %+v", saved[0])
	}
}
