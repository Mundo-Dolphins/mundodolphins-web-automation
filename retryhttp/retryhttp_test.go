package retryhttp

import (
	"context"
	"io"
	"net/http"
	"syscall"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestDoRetriesConnectionReset(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return nil, syscall.ECONNRESET
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(http.NoBody),
			Header:     make(http.Header),
		}, nil
	})}

	resp, err := Do(context.Background(), client, Policy{MaxRetries: 1}, func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestDefaultClientHasTimeout(t *testing.T) {
	if defaultClient.Timeout == 0 {
		t.Fatal("the fallback HTTP client must have a timeout")
	}
}
