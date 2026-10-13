package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEnsurePullRequestReusesExisting(t *testing.T) {
	var requests []string
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("Authorization header = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls":
			return jsonResponse(http.StatusOK, []PullRequest{{Number: 7, HTMLURL: "https://example/pr/7"}}), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues/7/labels":
			return jsonResponse(http.StatusOK, map[string]string{}), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})})

	pr, created, err := client.EnsurePullRequest(context.Background(), "title", "body", "work", "main", "automerge")
	if err != nil {
		t.Fatalf("EnsurePullRequest() error = %v", err)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	if pr.Number != 7 {
		t.Fatalf("pr.Number = %d, want 7", pr.Number)
	}
	want := []string{
		"GET /repos/owner/repo/pulls",
		"POST /repos/owner/repo/issues/7/labels",
	}
	if len(requests) != len(want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("request[%d] = %q, want %q", i, requests[i], want[i])
		}
	}
}

func TestEnsurePullRequestCreatesAndLabelsMissingPR(t *testing.T) {
	var requests []string
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls":
			return jsonResponse(http.StatusOK, []PullRequest{}), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/pulls":
			return jsonResponse(http.StatusOK, PullRequest{Number: 9, HTMLURL: "https://example/pr/9"}), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues/9/labels":
			return jsonResponse(http.StatusOK, map[string]string{}), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})})

	pr, created, err := client.EnsurePullRequest(context.Background(), "title", "body", "work", "main", "automerge")
	if err != nil {
		t.Fatalf("EnsurePullRequest() error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if pr.Number != 9 {
		t.Fatalf("pr.Number = %d, want 9", pr.Number)
	}

	want := []string{
		"GET /repos/owner/repo/pulls",
		"POST /repos/owner/repo/pulls",
		"POST /repos/owner/repo/issues/9/labels",
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("request[%d] = %q, want %q", i, requests[i], want[i])
		}
	}
}

func TestCreatePullRequestReturnsNoCommitsBetweenError(t *testing.T) {
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/owner/repo/pulls" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		return jsonResponse(http.StatusUnprocessableEntity, map[string]any{
			"message": "Validation Failed",
			"errors": []map[string]string{
				{
					"resource": "PullRequest",
					"code":     "custom",
					"message":  "No commits between main and new-podcast-episodes",
				},
			},
		}), nil
	})})

	_, err := client.CreatePullRequest(context.Background(), "title", "body", "work", "main")
	if !errors.Is(err, ErrNoCommitsBetween) {
		t.Fatalf("CreatePullRequest() error = %v, want ErrNoCommitsBetween", err)
	}
	if !IsNoCommitsBetween(err) {
		t.Fatal("IsNoCommitsBetween() = false, want true")
	}
}

func TestGitHubRetries429(t *testing.T) {
	requests := 0
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(http.StatusTooManyRequests, map[string]string{"message": "rate limited"}), nil
		}
		return jsonResponse(http.StatusOK, []PullRequest{{Number: 7, HTMLURL: "https://example/pr/7"}}), nil
	})})
	client.RetryDelay = 0

	prs, err := client.ListOpenPullRequests(context.Background(), "work", "main")
	if err != nil {
		t.Fatalf("ListOpenPullRequests() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(prs) != 1 {
		t.Fatalf("prs = %d, want 1", len(prs))
	}
}

func TestGitHubRetries5xxAndReturnsFinalError(t *testing.T) {
	requests := 0
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusBadGateway, map[string]string{"message": "upstream down"}), nil
	})})
	client.RetryCount = 2
	client.RetryDelay = 0

	_, err := client.ListOpenPullRequests(context.Background(), "work", "main")
	if err == nil {
		t.Fatal("expected error")
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestGitHubTimeoutIsControlled(t *testing.T) {
	client := New("owner", "repo", "secret-token", &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	client.Timeout = time.Millisecond
	client.RetryCount = 1
	client.RetryDelay = 0

	_, err := client.ListOpenPullRequests(context.Background(), "work", "main")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, payload any) *http.Response {
	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(payload)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(&body),
		Header:     make(http.Header),
	}
}

func TestOwnerAndRepoAreEscapedInThePath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	c := New("evil/owner", "repo?x=1", "token", srv.Client())
	c.BaseURL = srv.URL
	if _, err := c.ListOpenPullRequests(context.Background(), "head", "main"); err != nil {
		t.Fatal(err)
	}
	if want := "/repos/evil%2Fowner/repo%3Fx=1/pulls"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}
