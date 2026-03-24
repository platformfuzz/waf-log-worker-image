package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/platformfuzz/waf-log-worker-image/internal/config"
	"github.com/platformfuzz/waf-log-worker-image/internal/metrics"
	"github.com/platformfuzz/waf-log-worker-image/internal/runtime"
	lokisink "github.com/platformfuzz/waf-log-worker-image/internal/sink/loki"
	s3src "github.com/platformfuzz/waf-log-worker-image/internal/source/s3"
	sqssrc "github.com/platformfuzz/waf-log-worker-image/internal/source/sqs"
	"github.com/platformfuzz/waf-log-worker-image/internal/transform/waf"
)

type s3EventEnvelope struct {
	Records []struct {
		S3 struct {
			Bucket struct {
				Name string `json:"name"`
			} `json:"bucket"`
			Object struct {
				Key string `json:"key"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

// Run starts worker poll loops and processes S3-backed WAF events from SQS.
func Run(ctx context.Context, cfg config.Config) error {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return err
	}
	sqsClient := sqssrc.New(awssqs.NewFromConfig(awsCfg), cfg.SQSQueueURL)
	s3Client := s3src.New(s3.NewFromConfig(awsCfg))
	lokiClient := &lokisink.Client{
		URL:      cfg.LokiURL,
		TenantID: cfg.LokiTenantID,
		HTTP: &http.Client{
			Timeout: cfg.LokiPushTimeout,
		},
		MaxRetries: cfg.LokiMaxRetries,
	}

	tfm := waf.Transformer{
		ACLAllowlist:          cfg.WafACLAllowlist,
		ActionAllowlist:       cfg.WafActionAllowlist,
		SampleAllowPercent:    cfg.SampleAllowPercent,
		EnableGeoIP:           cfg.EnableGeoIP,
		EnableCountryCentroid: cfg.EnableCountryCentroid,
	}

	m := &metrics.Counters{}
	stopMetrics := make(chan struct{})
	defer close(stopMetrics)
	go metrics.StartLogger(m, 30*time.Second, stopMetrics)

	return runtime.RunWorkers(ctx, cfg.WorkerConcurrency, func(workerCtx context.Context) error {
		for workerCtx.Err() == nil {
			msgs, recvErr := sqsClient.Receive(workerCtx, cfg.PollMaxMessages, cfg.PollWaitSeconds)
			if recvErr != nil {
				m.IncErr()
				log.Printf("receive error: %v", recvErr)
				time.Sleep(2 * time.Second)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			for _, msg := range msgs {
				m.IncMessagesReceived()
				if msg.Body == nil {
					continue
				}
				if err := processMessage(workerCtx, cfg, *msg.Body, tfm, s3Client, lokiClient, m); err != nil {
					m.IncErr()
					log.Printf("process message failed: %v", err)
					continue
				}
				if msg.ReceiptHandle != nil {
					if delErr := sqsClient.Delete(workerCtx, *msg.ReceiptHandle); delErr != nil {
						m.IncErr()
						log.Printf("delete message failed: %v", delErr)
						continue
					}
					m.IncMessagesDeleted()
				}
			}
		}
		return nil
	})
}

func processMessage(
	ctx context.Context,
	cfg config.Config,
	body string,
	tfm waf.Transformer,
	s3Client *s3src.Client,
	lokiClient *lokisink.Client,
	m *metrics.Counters,
) error {
	if strings.Contains(body, "\"Event\":\"s3:TestEvent\"") {
		return nil
	}

	var env s3EventEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		return fmt.Errorf("decode S3 envelope: %w", err)
	}
	for _, r := range env.Records {
		bucket := r.S3.Bucket.Name
		key := strings.ReplaceAll(r.S3.Object.Key, "+", " ")
		if bucket == "" || key == "" {
			continue
		}
		lines, err := s3Client.ReadLines(ctx, bucket, key)
		if err != nil {
			return fmt.Errorf("read s3 object bucket=%s key=%s: %w", bucket, key, err)
		}
		m.AddObjectsRead(1)
		m.AddRecordsRead(len(lines))
		acl := waf.WafACLNameFromS3Key(key)

		entries := make([]lokisink.Entry, 0, len(lines))
		for _, line := range lines {
			out, keep := tfm.Transform(line, acl)
			if !keep || strings.TrimSpace(out) == "" {
				m.AddRecordsDrop(1)
				continue
			}
			entries = append(entries, lokisink.Entry{
				TsNs: lokisink.TimestampNs(time.Now().UnixMilli()),
				Line: out,
			})
		}
		batches := chunk(entries, cfg.LokiBatchMaxLines)
		for _, b := range batches {
			dropped, pushErr := lokiClient.Push(ctx, map[string]string{
				"source":  "waf",
				"bucket":  bucket,
				"waf_acl": acl,
			}, b)
			if pushErr != nil {
				markLokiRateLimit(pushErr, m)
				return pushErr
			}
			if dropped > 0 {
				m.AddRecordsDrop(dropped)
			}
			m.AddRecordsPush(len(b) - dropped)
		}
	}
	return nil
}

func markLokiRateLimit(err error, m *metrics.Counters) {
	if strings.Contains(err.Error(), "status=429") {
		m.Inc429()
	}
}

func chunk(entries []lokisink.Entry, size int) [][]lokisink.Entry {
	if size <= 0 || len(entries) == 0 {
		return [][]lokisink.Entry{entries}
	}
	out := make([][]lokisink.Entry, 0, (len(entries)/size)+1)
	for i := 0; i < len(entries); i += size {
		j := i + size
		if j > len(entries) {
			j = len(entries)
		}
		out = append(out, entries[i:j])
	}
	return out
}
