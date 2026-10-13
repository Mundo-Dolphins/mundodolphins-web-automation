package social

import (
	"os"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
)

var xmls = map[string]string{
	"https://bsky.app/profile/alice.example.com/rss": "testdata/alice.xml",
	"https://bsky.app/profile/bob.example.com/rss":   "testdata/bob.xml",
}

func TestFetchFeeds(t *testing.T) {
	// Activate httpmock to intercept all HTTP calls
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)
	// Mock oEmbed responses for all item links in the XML files
	oembedResponse := `{"type":"rich","version":"1.0","author_name":"test","author_url":"https://bsky.app/profile/test","provider_url":"https://bsky.app/","cache_age":3600,"width":400,"height":null,"html":"<blockquote data-bluesky-cid=\"testcid\"></blockquote>"}`
	// For each XML file, parse the links and mock the oEmbed response
	for _, filePath := range xmls {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := string(fileContent)
		// Find all <link>...</link> entries for posts
		start := 0
		for {
			idx := strings.Index(content[start:], "<link>")
			if idx == -1 {
				break
			}
			idx += start
			endIdx := strings.Index(content[idx:], "</link>")
			if endIdx == -1 {
				break
			}
			endIdx += idx
			link := content[idx+6 : endIdx]
			if strings.Contains(link, "/post/") {
				oembedUrl := "https://embed.bsky.app/oembed?url=" + link
				httpmock.RegisterResponder("GET", oembedUrl, httpmock.NewStringResponder(200, oembedResponse))
			}
			start = endIdx + 7
		}
	}

	urls := []string{}
	for url, filePath := range xmls {
		urls = append(urls, url)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Error reading file %s: %v", filePath, err)
		}
		// Check that the XML contains at least one <item>
		if !strings.Contains(string(fileContent), "<item>") {
			t.Fatalf("File %s does not contain any <item>", filePath)
		}
		httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, string(fileContent)))
	}

	reader := Reader{UseBlueSkyRSSFallback: true}
	items, err := reader.fetchBlueSkyFeeds(nil, urls, []string{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("Expected 4 items (2 per profile), got %d. URLs: %v", len(items), urls)
	}
	for _, item := range items {
		if item.SType != BlueSky || item.BlueSkyPost.BskyCID != "testcid" {
			t.Fatalf("unexpected item: %+v", item)
		}
	}
}
