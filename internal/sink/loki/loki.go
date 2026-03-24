package loki

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client pushes batches to Loki.
type Client struct {
	URL        string
	TenantID   string
	HTTP       *http.Client
	MaxRetries int
}

// Entry represents one Loki log line with nanosecond timestamp.
type Entry struct {
	TsNs string
	Line string
}

// Push sends one stream batch and returns dropped record count on stale-entry responses.
func (c *Client) Push(ctx context.Context, labels map[string]string, entries []Entry) (dropped int, err error) {
	if len(entries) == 0 {
		return 0, nil
	}
	values := make([][2]string, 0, len(entries))
	for _, e := range entries {
		values = append(values, [2]string{e.TsNs, e.Line})
	}
	bodyObj := map[string]any{
		"streams": []any{
			map[string]any{
				"stream": labels,
				"values": values,
			},
		},
	}
	body, err := json.Marshal(bodyObj)
	if err != nil {
		return 0, err
	}

	for attempt := 1; attempt <= c.MaxRetries; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
		if reqErr != nil {
			return 0, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		if c.TenantID != "" {
			req.Header.Set("X-Scope-OrgID", c.TenantID)
		}

		res, doErr := c.HTTP.Do(req)
		if doErr != nil {
			if attempt == c.MaxRetries {
				return 0, doErr
			}
			sleepBackoff(attempt)
			continue
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return 0, nil
		}

		detail := string(b)
		if res.StatusCode == 400 && strings.Contains(detail, "entry too far behind") {
			return len(entries), nil
		}
		if res.StatusCode == 429 && attempt < c.MaxRetries {
			sleepBackoff(attempt)
			continue
		}
		return 0, fmt.Errorf("loki push failed status=%d body=%s", res.StatusCode, truncate(detail, 400))
	}
	return 0, fmt.Errorf("unreachable retry loop")
}

// TimestampNs converts epoch milliseconds to nanoseconds expected by Loki.
func TimestampNs(ms int64) string {
	if ms > math.MaxInt64/1_000_000 {
		return strconv.FormatInt(math.MaxInt64, 10)
	}
	if ms < math.MinInt64/1_000_000 {
		return strconv.FormatInt(math.MinInt64, 10)
	}
	return strconv.FormatInt(ms*1_000_000, 10)
}

func sleepBackoff(attempt int) {
	base := math.Pow(2, float64(attempt-1))
	jitter := 0.5 + secureJitter()
	time.Sleep(time.Duration(base*jitter*float64(time.Second)) * time.Nanosecond)
}

func secureJitter() float64 {
	// Best-effort crypto randomness; fallback to neutral jitter.
	n, err := rand.Int(rand.Reader, big.NewInt(10_000))
	if err != nil {
		return 0.5
	}
	return float64(n.Int64()) / 10_000.0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
