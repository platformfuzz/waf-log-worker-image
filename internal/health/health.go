package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// ListenAddrFromEnv returns the TCP listen address for the health server, or empty if disabled.
// Unset HEALTH_LISTEN_ADDR defaults to 0.0.0.0:8080. Set to "-" or "off" to disable.
func ListenAddrFromEnv() string {
	v := strings.TrimSpace(os.Getenv("HEALTH_LISTEN_ADDR"))
	if v == "" {
		return "0.0.0.0:8080"
	}
	if v == "-" || strings.EqualFold(v, "off") {
		return ""
	}
	return v
}

// Handler returns an HTTP handler for the standard liveness path /healthz:
// GET and HEAD return 200 when the process is up (Kubernetes-style httpGet-compatible).
// Other methods return 405. Response is plain text "ok" for GET; no body for HEAD.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			// Avoid intermediaries caching probe results (common on health endpoints).
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
		default:
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

// Start binds listenAddr and serves /healthz until ctx is canceled. Fails fast if bind fails.
func Start(ctx context.Context, listenAddr string) error {
	if strings.TrimSpace(listenAddr) == "" {
		return nil
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		// Parent ctx is already done; use WithoutCancel so the shutdown deadline still applies (gosec G118).
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

// ProbeExitCode runs a one-shot GET /healthz (for ECS HEALTHCHECK CMD exec; image has no shell/curl).
func ProbeExitCode() int {
	addr := ListenAddrFromEnv()
	if addr == "" {
		fmt.Fprintln(os.Stderr, "probe: health server disabled (HEALTH_LISTEN_ADDR)")
		return 1
	}
	target, err := probeURL(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		return 1
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: get %s: %v\n", target, err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "probe: %s status %d\n", target, resp.StatusCode)
		return 1
	}
	return 0
}

func probeURL(listenAddr string) (string, error) {
	addr := strings.TrimSpace(listenAddr)
	if strings.HasPrefix(addr, ":") {
		return fmt.Sprintf("http://127.0.0.1%s/healthz", addr), nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse listen address: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s/healthz", net.JoinHostPort(host, port)), nil
}
