package s3src

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps S3 object access used by the worker pipeline.
type Client struct {
	api *s3.Client
}

// New creates an S3 source client wrapper.
func New(api *s3.Client) *Client {
	return &Client{api: api}
}

// ReadLines downloads an object and returns non-empty line separated records.
func (c *Client) ReadLines(ctx context.Context, bucket, key string) ([]string, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := out.Body.Close()
		_ = closeErr
	}()

	var rdr io.Reader = out.Body
	if strings.HasSuffix(key, ".gz") || strings.EqualFold(aws.ToString(out.ContentEncoding), "gzip") {
		gzr, zerr := gzip.NewReader(out.Body)
		if zerr != nil {
			return nil, fmt.Errorf("gzip reader: %w", zerr)
		}
		defer func() {
			closeErr := gzr.Close()
			_ = closeErr
		}()
		rdr = gzr
	}

	buf, err := io.ReadAll(rdr)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(buf))
	lines := make([]string, 0, 256)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
