package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Client wraps SQS operations used by the worker pipeline.
type Client struct {
	api      *sqs.Client
	queueURL string
}

// New creates an SQS source client wrapper.
func New(api *sqs.Client, queueURL string) *Client {
	return &Client{api: api, queueURL: queueURL}
}

// Receive long-polls the queue and returns up to max messages.
func (c *Client) Receive(ctx context.Context, max, wait int32) ([]types.Message, error) {
	out, err := c.api.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: max,
		WaitTimeSeconds:     wait,
	})
	if err != nil {
		return nil, err
	}
	return out.Messages, nil
}

// Delete removes one message from the queue by receipt handle.
func (c *Client) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.api.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
