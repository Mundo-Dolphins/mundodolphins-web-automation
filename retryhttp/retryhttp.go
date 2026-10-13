package retryhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Policy struct {
	MaxRetries int
	Delay      time.Duration
}

// defaultClient is used when the caller passes none; unlike http.DefaultClient
// it has a timeout.
var defaultClient = &http.Client{Timeout: 30 * time.Second}

func Do(ctx context.Context, client Doer, policy Policy, newRequest func(context.Context) (*http.Request, error)) (*http.Response, error) {
	if client == nil {
		client = defaultClient
	}

	var lastErr error
	attempts := policy.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 0; attempt < attempts; attempt++ {
		req, err := newRequest(ctx)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err == nil && !RetryableStatus(resp.StatusCode) {
			return resp, nil
		}
		if resp != nil {
			lastErr = fmt.Errorf("retryable HTTP status %d", resp.StatusCode)
			_ = resp.Body.Close()
		} else {
			lastErr = err
		}

		if attempt == attempts-1 || !RetryableError(ctx, lastErr) && resp == nil {
			break
		}
		if waitErr := sleep(ctx, policy.Delay); waitErr != nil {
			return nil, waitErr
		}
	}

	return nil, lastErr
}

func RetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500 && status <= 599
}

func RetryableError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") || strings.Contains(msg, "timeout")
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
