package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"vaws/internal/log"
	"vaws/internal/model"
)

// maxConcurrentSQSCalls limits concurrent API calls to avoid throttling
const maxConcurrentSQSCalls = 10

// redrivePolicy represents the JSON structure of SQS RedrivePolicy.
type redrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     int    `json:"maxReceiveCount"`
}

// ListQueues returns all SQS queues in the region with their attributes.
func (c *Client) ListQueues(ctx context.Context) ([]model.Queue, error) {
	log.Debug("Listing SQS queues...")

	var queueURLs []string
	paginator := sqs.NewListQueuesPaginator(c.sqs, &sqs.ListQueuesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list queues: %w", err)
		}
		queueURLs = append(queueURLs, page.QueueUrls...)
	}

	if len(queueURLs) == 0 {
		log.Info("No SQS queues found")
		return nil, nil
	}

	log.Debug("Found %d queue URLs, fetching attributes in parallel...", len(queueURLs))

	// Fetch queue attributes in parallel
	type queueResult struct {
		index int
		queue *model.Queue
		err   error
	}

	results := make(chan queueResult, len(queueURLs))
	sem := make(chan struct{}, maxConcurrentSQSCalls) // Limit concurrency

	var wg sync.WaitGroup
	for i, url := range queueURLs {
		wg.Add(1)
		go func(idx int, queueURL string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			queue, err := c.GetQueueAttributes(ctx, queueURL)
			results <- queueResult{index: idx, queue: queue, err: err}
		}(i, url)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	queues := make([]model.Queue, len(queueURLs))
	validCount := 0

	for result := range results {
		if result.err != nil {
			log.Warn("Failed to get attributes for queue: %v", result.err)
			continue
		}
		queues[result.index] = *result.queue
		validCount++
	}

	// Filter out empty entries (failed fetches)
	validQueues := make([]model.Queue, 0, validCount)
	for _, q := range queues {
		if q.URL != "" {
			validQueues = append(validQueues, q)
		}
	}

	// Sort queues alphabetically by name
	sort.Slice(validQueues, func(i, j int) bool {
		return strings.ToLower(validQueues[i].Name) < strings.ToLower(validQueues[j].Name)
	})

	log.Info("Found %d SQS queues", len(validQueues))
	return validQueues, nil
}

// GetQueueAttributes returns detailed information about a specific queue.
func (c *Client) GetQueueAttributes(ctx context.Context, queueURL string) (*model.Queue, error) {
	log.Debug("Getting attributes for SQS queue: %s", queueURL)

	out, err := c.sqs.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get queue attributes: %w", err)
	}

	return convertQueueAttributes(queueURL, out.Attributes), nil
}

// GetQueuesFromStack returns SQS queue URLs from a CloudFormation stack.
func (c *Client) GetQueuesFromStack(ctx context.Context, stackName string) ([]string, error) {
	log.Debug("Getting SQS queues from stack: %s", stackName)

	resources, err := c.GetStackResources(ctx, stackName, "AWS::SQS::Queue")
	if err != nil {
		return nil, err
	}

	var queueURLs []string
	for _, r := range resources {
		if r.PhysicalResourceId != nil {
			queueURLs = append(queueURLs, *r.PhysicalResourceId)
		}
	}

	log.Debug("Found %d SQS queues in stack %s", len(queueURLs), stackName)
	return queueURLs, nil
}

// convertQueueAttributes converts AWS SQS attributes to our model.
func convertQueueAttributes(url string, attrs map[string]string) *model.Queue {
	queue := &model.Queue{
		URL:  url,
		Name: extractQueueNameFromURL(url),
		Type: model.QueueTypeStandard,
	}

	// Parse ARN
	if arn, ok := attrs[string(sqstypes.QueueAttributeNameQueueArn)]; ok {
		queue.ARN = arn
	}

	// Determine queue type from name (FIFO queues end with .fifo)
	if strings.HasSuffix(queue.Name, ".fifo") {
		queue.Type = model.QueueTypeFIFO
	}

	// Parse message counts
	if val, ok := attrs[string(sqstypes.QueueAttributeNameApproximateNumberOfMessages)]; ok {
		queue.ApproximateMessageCount, _ = strconv.Atoi(val)
	}
	if val, ok := attrs[string(sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible)]; ok {
		queue.ApproximateInFlight, _ = strconv.Atoi(val)
	}

	// Parse configuration
	if val, ok := attrs[string(sqstypes.QueueAttributeNameVisibilityTimeout)]; ok {
		queue.VisibilityTimeout, _ = strconv.Atoi(val)
	}
	if val, ok := attrs[string(sqstypes.QueueAttributeNameMessageRetentionPeriod)]; ok {
		queue.MessageRetentionPeriod, _ = strconv.Atoi(val)
	}
	if val, ok := attrs[string(sqstypes.QueueAttributeNameDelaySeconds)]; ok {
		queue.DelaySeconds, _ = strconv.Atoi(val)
	}

	// Parse created timestamp
	if val, ok := attrs[string(sqstypes.QueueAttributeNameCreatedTimestamp)]; ok {
		if ts, err := strconv.ParseInt(val, 10, 64); err == nil {
			queue.CreatedAt = time.Unix(ts, 0)
		}
	}

	// Parse redrive policy (DLQ info)
	if val, ok := attrs[string(sqstypes.QueueAttributeNameRedrivePolicy)]; ok && val != "" {
		var policy redrivePolicy
		if err := json.Unmarshal([]byte(val), &policy); err == nil {
			if policy.DeadLetterTargetArn != "" {
				queue.HasDLQ = true
				queue.DLQArn = policy.DeadLetterTargetArn
				queue.MaxReceiveCount = policy.MaxReceiveCount
			}
		}
	}

	return queue
}

// extractQueueNameFromURL extracts the queue name from its URL.
// URL format: https://sqs.{region}.amazonaws.com/{account}/{queue-name}
func extractQueueNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// ListQueuesPagedCallback lists SQS queues with a callback for each batch.
// This enables lazy loading by delivering results incrementally.
// The callback receives queues from each batch and returns true to continue or false to stop.
func (c *Client) ListQueuesPagedCallback(ctx context.Context, callback func(queues []model.Queue, hasMore bool) bool) error {
	log.Debug("Listing SQS queues with lazy loading...")

	// Use smaller page size for responsive incremental loading
	paginator := sqs.NewListQueuesPaginator(c.sqs, &sqs.ListQueuesInput{
		MaxResults: aws.Int32(25),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list queues: %w", err)
		}

		if len(page.QueueUrls) == 0 {
			continue
		}

		// Fetch attributes for this batch of queue URLs
		queues := c.fetchQueueAttributesBatch(ctx, page.QueueUrls)

		hasMore := paginator.HasMorePages()
		if !callback(queues, hasMore) {
			break
		}
	}

	return nil
}

// fetchQueueAttributesBatch fetches attributes for a batch of queue URLs in parallel.
func (c *Client) fetchQueueAttributesBatch(ctx context.Context, queueURLs []string) []model.Queue {
	type queueResult struct {
		index int
		queue *model.Queue
		err   error
	}

	results := make(chan queueResult, len(queueURLs))
	sem := make(chan struct{}, maxConcurrentSQSCalls)

	var wg sync.WaitGroup
	for i, url := range queueURLs {
		wg.Add(1)
		go func(idx int, queueURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			queue, err := c.GetQueueAttributes(ctx, queueURL)
			results <- queueResult{index: idx, queue: queue, err: err}
		}(i, url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results maintaining order
	queues := make([]model.Queue, len(queueURLs))
	validCount := 0

	for result := range results {
		if result.err != nil {
			log.Warn("Failed to get attributes for queue: %v", result.err)
			continue
		}
		queues[result.index] = *result.queue
		validCount++
	}

	// Filter out empty entries
	validQueues := make([]model.Queue, 0, validCount)
	for _, q := range queues {
		if q.URL != "" {
			validQueues = append(validQueues, q)
		}
	}

	// Sort batch alphabetically
	sort.Slice(validQueues, func(i, j int) bool {
		return strings.ToLower(validQueues[i].Name) < strings.ToLower(validQueues[j].Name)
	})

	return validQueues
}
