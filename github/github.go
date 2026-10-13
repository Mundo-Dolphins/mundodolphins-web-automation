package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-web-automation/retryhttp"
)

const defaultBaseURL = "https://api.github.com"

// maxResponseBytes bounds how much of an API response is decoded.
const maxResponseBytes = 8 << 20

const (
	defaultGitHubTimeout    = 20 * time.Second
	defaultGitHubRetryCount = 2
	defaultGitHubRetryDelay = time.Second
)

var ErrNoCommitsBetween = errors.New("github pull request has no commits between base and head")

type Client struct {
	BaseURL    string
	Owner      string
	Repo       string
	Token      string
	HTTPClient *http.Client
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
}

type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	HeadRef string `json:"head_ref,omitempty"`
	BaseRef string `json:"base_ref,omitempty"`
}

func New(owner string, repo string, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultGitHubTimeout}
	}
	return &Client{
		BaseURL:    defaultBaseURL,
		Owner:      owner,
		Repo:       repo,
		Token:      token,
		HTTPClient: httpClient,
		Timeout:    defaultGitHubTimeout,
		RetryCount: defaultGitHubRetryCount,
		RetryDelay: defaultGitHubRetryDelay,
	}
}

func (c *Client) ListOpenPullRequests(ctx context.Context, head string, base string) ([]PullRequest, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", c.Owner+":"+head)
	query.Set("base", base)
	var prs []PullRequest
	err := c.do(ctx, http.MethodGet, "/repos/"+url.PathEscape(c.Owner)+"/"+url.PathEscape(c.Repo)+"/pulls?"+query.Encode(), nil, &prs)
	return prs, err
}

func (c *Client) CreatePullRequest(ctx context.Context, title string, body string, head string, base string) (PullRequest, error) {
	payload := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}
	var pr PullRequest
	err := c.do(ctx, http.MethodPost, "/repos/"+url.PathEscape(c.Owner)+"/"+url.PathEscape(c.Repo)+"/pulls", payload, &pr)
	if isNoCommitsBetweenError(err) {
		return PullRequest{}, ErrNoCommitsBetween
	}
	return pr, err
}

func (c *Client) AddLabel(ctx context.Context, issueNumber int, label string) error {
	payload := map[string][]string{"labels": {label}}
	return c.do(ctx, http.MethodPost, "/repos/"+url.PathEscape(c.Owner)+"/"+url.PathEscape(c.Repo)+"/issues/"+strconv.Itoa(issueNumber)+"/labels", payload, nil)
}

func (c *Client) EnsurePullRequest(ctx context.Context, title string, body string, head string, base string, label string) (PullRequest, bool, error) {
	prs, err := c.ListOpenPullRequests(ctx, head, base)
	if err != nil {
		return PullRequest{}, false, err
	}
	if len(prs) > 0 {
		if label != "" {
			if err := c.AddLabel(ctx, prs[0].Number, label); err != nil {
				return PullRequest{}, false, err
			}
		}
		return prs[0], false, nil
	}

	pr, err := c.CreatePullRequest(ctx, title, body, head, base)
	if err != nil {
		return PullRequest{}, false, err
	}
	if label != "" {
		if err := c.AddLabel(ctx, pr.Number, label); err != nil {
			return PullRequest{}, false, err
		}
	}
	return pr, true, nil
}

func (c *Client) do(ctx context.Context, method string, path string, payload any, target any) error {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	var raw []byte
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultGitHubTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := retryhttp.Do(runCtx, c.HTTPClient, retryhttp.Policy{MaxRetries: c.retryCount(), Delay: c.RetryDelay}, func(reqCtx context.Context) (*http.Request, error) {
		var body io.Reader
		if raw != nil {
			body = bytes.NewReader(raw)
		}
		req, err := http.NewRequestWithContext(reqCtx, method, baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("github %s %s failed: %w", method, pathWithoutQuery(path), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("github %s %s returned status %d: %s", method, pathWithoutQuery(path), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(target)
}

func (c *Client) retryCount() int {
	if c.RetryCount > 0 {
		return c.RetryCount
	}
	return defaultGitHubRetryCount
}

func pathWithoutQuery(path string) string {
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		return path[:idx]
	}
	return path
}

func IsNoCommitsBetween(err error) bool {
	return errors.Is(err, ErrNoCommitsBetween)
}

func isNoCommitsBetweenError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "returned status 422") &&
		strings.Contains(message, "no commits between")
}
