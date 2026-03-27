package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ioReadAllString(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	return string(b), err
}

func TestHandler_Healthz_GET(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got == "" {
		t.Error("expected Cache-Control on /healthz")
	}
	body, err := ioReadAllString(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body != "ok\n" {
		t.Fatalf("body %q, want \"ok\\n\"", body)
	}
}

func TestHandler_Healthz_HEAD(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodHead, srv.URL+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, err := ioReadAllString(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		t.Fatalf("HEAD body should be empty, got %q", body)
	}
}

func TestHandler_Healthz_POST(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/healthz", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405", resp.StatusCode)
	}
	if g := resp.Header.Get("Allow"); g != "GET, HEAD" {
		t.Fatalf("Allow %q", g)
	}
}

func TestProbeURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{":9090", "http://127.0.0.1:9090/healthz"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/healthz"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080/healthz"},
	}
	for _, tt := range tests {
		got, err := probeURL(tt.in)
		if err != nil {
			t.Fatalf("probeURL(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("probeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
