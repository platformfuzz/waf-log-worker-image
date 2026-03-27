package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/platformfuzz/waf-log-worker-image/internal/health"
)

// Config stores runtime settings for the worker process.
type Config struct {
	AWSRegion                         string
	SQSQueueURL                       string
	LokiURL                           string
	LokiTenantID                      string
	PollWaitSeconds                   int32
	PollMaxMessages                   int32
	WorkerConcurrency                 int
	S3ReadTimeout                     time.Duration
	LokiPushTimeout                   time.Duration
	LokiBatchMaxBytes                 int
	LokiBatchMaxLines                 int
	LokiMaxRetries                    int
	MaxInflightPushes                 int
	MessageVisibilityHeartbeatSeconds int
	ShutdownGraceSeconds              int
	WafACLAllowlist                   map[string]bool
	WafActionAllowlist                map[string]bool
	SampleAllowPercent                int
	EnableGeoIP                       bool
	EnableCountryCentroid             bool
	HealthListenAddr                  string
}

// Load reads environment variables and returns runtime configuration.
func Load() (Config, error) {
	cfg := Config{
		AWSRegion:                         getEnv("AWS_REGION", "ap-southeast-2"),
		SQSQueueURL:                       strings.TrimSpace(os.Getenv("SQS_QUEUE_URL")),
		LokiURL:                           strings.TrimSpace(os.Getenv("LOKI_URL")),
		LokiTenantID:                      strings.TrimSpace(os.Getenv("LOKI_TENANT_ID")),
		PollWaitSeconds:                   getEnvInt32("POLL_WAIT_SECONDS", 20),
		PollMaxMessages:                   getEnvInt32("POLL_MAX_MESSAGES", 10),
		WorkerConcurrency:                 getEnvInt("WORKER_CONCURRENCY", 2),
		S3ReadTimeout:                     time.Duration(getEnvInt("S3_READ_TIMEOUT_SECONDS", 60)) * time.Second,
		LokiPushTimeout:                   time.Duration(getEnvInt("LOKI_PUSH_TIMEOUT_SECONDS", 15)) * time.Second,
		LokiBatchMaxBytes:                 getEnvInt("LOKI_BATCH_MAX_BYTES", 200_000),
		LokiBatchMaxLines:                 getEnvInt("LOKI_BATCH_MAX_LINES", 200),
		LokiMaxRetries:                    getEnvInt("LOKI_MAX_RETRIES", 4),
		MaxInflightPushes:                 getEnvInt("MAX_INFLIGHT_PUSHES", 4),
		MessageVisibilityHeartbeatSeconds: getEnvInt("MESSAGE_VISIBILITY_HEARTBEAT_SECONDS", 30),
		ShutdownGraceSeconds:              getEnvInt("SHUTDOWN_GRACE_SECONDS", 20),
		WafACLAllowlist:                   csvSet("WAF_ACL_ALLOWLIST"),
		WafActionAllowlist:                csvSet("WAF_ACTION_ALLOWLIST"),
		SampleAllowPercent:                getEnvInt("SAMPLE_ALLOW_PERCENT", 100),
		EnableGeoIP:                       getEnvBool("ENABLE_GEOIP", false),
		EnableCountryCentroid:             getEnvBool("ENABLE_COUNTRY_CENTROID", true),
		HealthListenAddr:                  health.ListenAddrFromEnv(),
	}

	if cfg.SQSQueueURL == "" {
		return cfg, fmt.Errorf("SQS_QUEUE_URL is required")
	}
	if cfg.LokiURL == "" {
		return cfg, fmt.Errorf("LOKI_URL is required")
	}
	return cfg, nil
}

func getEnv(name, def string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	return v
}

func getEnvInt(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvInt32(name string, def int32) int32 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n64, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return def
	}
	if n64 > math.MaxInt32 {
		return math.MaxInt32
	}
	if n64 < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n64)
}

func getEnvBool(name string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}

func csvSet(name string) map[string]bool {
	out := map[string]bool{}
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return out
	}
	for _, v := range strings.Split(raw, ",") {
		s := strings.TrimSpace(v)
		if s != "" {
			out[s] = true
		}
	}
	return out
}
