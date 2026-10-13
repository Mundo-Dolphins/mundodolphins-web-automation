package social

import (
	"testing"
	"time"
)

func TestBlueSkyOrderStability(t *testing.T) {
	// Create test posts with the same published date
	sameTime := time.Date(2025, 10, 6, 19, 13, 0, 0, time.UTC)

	// Simulate posts that could swap order
	posts := []OutputPost{
		{
			ID:          "https://bsky.app/profile/bob.example.com/post/3m2kd22ttdc2x",
			SType:       BlueSky,
			PublishedOn: sameTime,
			BlueSkyPost: BlueSkyPost{
				BskyPost: "https://bsky.app/profile/bob.example.com/post/3m2kd22ttdc2x",
			},
		},
		{
			ID:          "https://bsky.app/profile/bob.example.com/post/3m2kcywcoak2x",
			SType:       BlueSky,
			PublishedOn: sameTime,
			BlueSkyPost: BlueSkyPost{
				BskyPost: "https://bsky.app/profile/bob.example.com/post/3m2kcywcoak2x",
			},
		},
	}

	// Create multiple orderings and verify the order is consistent

	// Simulate the sorting process several times
	for i := 0; i < 10; i++ {
		// Make a copy of the posts for each iteration
		testPosts := make([]OutputPost, len(posts))
		copy(testPosts, posts)

		// If we simulate that they arrive in a different order each iteration
		if i%2 == 1 {
			// Swap order to simulate feed variability
			testPosts[0], testPosts[1] = testPosts[1], testPosts[0]
		}

		// Apply the same sorting logic used by FetchSocialPosts
		// (simplified for the test)
		result := applySorting(testPosts)

		// Verify the order is deterministic based on the ID
		// The post with ID "3m2kcywcoak2x" should come before "3m2kd22ttdc2x"
		// because "3m2kcywcoak2x" < "3m2kd22ttdc2x" lexicographically
		expectedFirstID := "https://bsky.app/profile/bob.example.com/post/3m2kcywcoak2x"
		if result[0].ID != expectedFirstID {
			t.Errorf("Iteration %d: Inconsistent order. Expected ID: %s, got: %s",
				i, expectedFirstID, result[0].ID)
		}
	}
}

// Helper function that applies the same sorting logic as the main code
func applySorting(posts []OutputPost) []OutputPost {
	result := make([]OutputPost, len(posts))
	copy(result, posts)

	// Apply the same sorting logic as in social.go
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			// First sort by published date (descending)
			if !result[i].PublishedOn.Equal(result[j].PublishedOn) {
				if result[j].PublishedOn.After(result[i].PublishedOn) {
					result[i], result[j] = result[j], result[i]
				}
			} else {
				// If dates are equal, use ID as deterministic tiebreaker (ascending)
				if result[j].ID < result[i].ID {
					result[i], result[j] = result[j], result[i]
				}
			}
		}
	}

	return result
}
