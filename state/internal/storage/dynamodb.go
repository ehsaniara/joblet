package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// dynamoDBBackend implements Backend using AWS DynamoDB
type dynamoDBBackend struct {
	client    DynamoDBAPI
	tableName string
	ttlDays   int
}

// NewDynamoDBBackend creates a new DynamoDB storage backend
func NewDynamoDBBackend(cfg *DynamoDBConfig) (Backend, error) {
	if cfg == nil {
		return nil, fmt.Errorf("DynamoDB configuration is required")
	}

	ctx := context.Background()

	// Load AWS config
	awsCfg, err := loadAWSConfig(ctx, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(awsCfg)

	backend := &dynamoDBBackend{
		client:    client,
		tableName: cfg.TableName,
		ttlDays:   cfg.TTLDays,
	}

	// Verify table exists
	if err := backend.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("table health check failed: %w", err)
	}

	return backend, nil
}

// NewDynamoDBBackendWithClient creates a DynamoDB backend with an injected client (for testing)
func NewDynamoDBBackendWithClient(client DynamoDBAPI, tableName string, ttlDays int) Backend {
	return &dynamoDBBackend{
		client:    client,
		tableName: tableName,
		ttlDays:   ttlDays,
	}
}

func (d *dynamoDBBackend) Create(ctx context.Context, job *domain.Job) error {
	// Set timestamp if not set
	if job.StartTime.IsZero() {
		job.StartTime = time.Now()
	}

	// Calculate TTL if enabled
	item := jobToItem(job, d.ttlDays)

	// PutItem with condition: jobId must not exist
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(d.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(jobId)"),
	}

	_, err := d.client.PutItem(ctx, input)
	if err != nil {
		if _, ok := err.(*types.ConditionalCheckFailedException); ok {
			return ErrJobAlreadyExists
		}
		return &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to create job", Err: err}
	}

	return nil
}

func (d *dynamoDBBackend) Get(ctx context.Context, jobID string) (*domain.Job, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"jobId": &types.AttributeValueMemberS{Value: jobID},
		},
	}

	result, err := d.client.GetItem(ctx, input)
	if err != nil {
		return nil, &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to get job", Err: err}
	}

	if result.Item == nil {
		return nil, ErrJobNotFound
	}

	job, err := itemToJob(result.Item)
	if err != nil {
		return nil, &StorageError{Code: "UNMARSHAL_ERROR", Message: "failed to unmarshal job", Err: err}
	}

	return job, nil
}

func (d *dynamoDBBackend) Update(ctx context.Context, job *domain.Job) error {
	// Calculate TTL
	item := jobToItem(job, d.ttlDays)

	// PutItem with condition: jobId must exist
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(d.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(jobId)"),
	}

	_, err := d.client.PutItem(ctx, input)
	if err != nil {
		if _, ok := err.(*types.ConditionalCheckFailedException); ok {
			return ErrJobNotFound
		}
		return &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to update job", Err: err}
	}

	return nil
}

func (d *dynamoDBBackend) Delete(ctx context.Context, jobID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"jobId": &types.AttributeValueMemberS{Value: jobID},
		},
	}

	_, err := d.client.DeleteItem(ctx, input)
	if err != nil {
		return &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to delete job", Err: err}
	}

	return nil
}

func (d *dynamoDBBackend) List(ctx context.Context, filter *Filter) ([]*domain.Job, error) {
	var jobs []*domain.Job

	// Use Scan for simple cases (TODO: optimize with GSI for status filtering)
	input := &dynamodb.ScanInput{
		TableName: aws.String(d.tableName),
	}

	// Apply filter expression if status is specified
	if filter != nil && filter.Status != "" {
		input.FilterExpression = aws.String("jobStatus = :status")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: filter.Status},
		}
	}

	// Apply limit
	if filter != nil && filter.Limit > 0 {
		input.Limit = aws.Int32(int32(filter.Limit))
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to scan jobs", Err: err}
	}

	// Convert items to jobs
	for _, item := range result.Items {
		job, err := itemToJob(item)
		if err != nil {
			// Log error but continue with other items
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (d *dynamoDBBackend) Sync(ctx context.Context, jobs []*domain.Job) error {
	// Batch write jobs (max 25 items per batch)
	const batchSize = 25

	for i := 0; i < len(jobs); i += batchSize {
		end := i + batchSize
		if end > len(jobs) {
			end = len(jobs)
		}

		batch := jobs[i:end]
		if err := d.writeBatch(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

func (d *dynamoDBBackend) writeBatch(ctx context.Context, jobs []*domain.Job) error {
	writeRequests := make([]types.WriteRequest, 0, len(jobs))

	for _, job := range jobs {
		item := jobToItem(job, d.ttlDays)
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			d.tableName: writeRequests,
		},
	}

	_, err := d.client.BatchWriteItem(ctx, input)
	if err != nil {
		return &StorageError{Code: "DYNAMODB_ERROR", Message: "failed to batch write", Err: err}
	}

	return nil
}

func (d *dynamoDBBackend) Close() error {
	// No cleanup needed for DynamoDB client
	return nil
}

func (d *dynamoDBBackend) HealthCheck(ctx context.Context) error {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	}

	_, err := d.client.DescribeTable(ctx, input)
	if err != nil {
		return &StorageError{Code: "TABLE_NOT_FOUND", Message: "DynamoDB table not accessible", Err: err}
	}

	return nil
}

// Helper functions

func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	// Auto-detect region from EC2 metadata if not specified
	if region == "" {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err == nil {
			imdsClient := imds.NewFromConfig(cfg)
			regionResp, err := imdsClient.GetRegion(ctx, &imds.GetRegionInput{})
			if err == nil {
				region = regionResp.Region
			}
		}
	}

	// Load AWS config with optional region
	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

func jobToItem(job *domain.Job, ttlDays int) map[string]types.AttributeValue {
	item := map[string]types.AttributeValue{
		"jobId":     &types.AttributeValueMemberS{Value: job.Uuid},
		"jobStatus": &types.AttributeValueMemberS{Value: string(job.Status)},
		"command":   &types.AttributeValueMemberS{Value: job.Command},
		"nodeId":    &types.AttributeValueMemberS{Value: job.NodeId},
	}

	// Timestamps
	if !job.StartTime.IsZero() {
		item["startTime"] = &types.AttributeValueMemberS{Value: job.StartTime.Format(time.RFC3339)}
	}
	if job.EndTime != nil && !job.EndTime.IsZero() {
		item["endTime"] = &types.AttributeValueMemberS{Value: job.EndTime.Format(time.RFC3339)}
	}
	if job.ScheduledTime != nil && !job.ScheduledTime.IsZero() {
		item["scheduledTime"] = &types.AttributeValueMemberS{Value: job.ScheduledTime.Format(time.RFC3339)}
	}

	// Optional fields
	if job.ExitCode != 0 {
		item["exitCode"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", job.ExitCode)}
	}
	if job.Pid != 0 {
		item["pid"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", job.Pid)}
	}

	// Infrastructure fields
	if job.Network != "" {
		item["network"] = &types.AttributeValueMemberS{Value: job.Network}
	}
	if job.Runtime != "" {
		item["runtime"] = &types.AttributeValueMemberS{Value: job.Runtime}
	}

	// TTL attribute (Unix timestamp when item should expire)
	if ttlDays > 0 && (job.Status == "COMPLETED" || job.Status == "FAILED") {
		expiresAt := time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour).Unix()
		item["expiresAt"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)}
	}

	return item
}

func itemToJob(item map[string]types.AttributeValue) (*domain.Job, error) {
	job := &domain.Job{}

	// Parse required fields
	if v, ok := item["jobId"].(*types.AttributeValueMemberS); ok {
		job.Uuid = v.Value
	}
	if v, ok := item["jobStatus"].(*types.AttributeValueMemberS); ok {
		job.Status = domain.JobStatus(v.Value)
	}
	if v, ok := item["command"].(*types.AttributeValueMemberS); ok {
		job.Command = v.Value
	}
	if v, ok := item["nodeId"].(*types.AttributeValueMemberS); ok {
		job.NodeId = v.Value
	}

	// Parse timestamps
	if v, ok := item["startTime"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			job.StartTime = t
		}
	}
	if v, ok := item["endTime"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			job.EndTime = &t
		}
	}
	if v, ok := item["scheduledTime"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			job.ScheduledTime = &t
		}
	}

	// Parse optional numeric fields
	if v, ok := item["exitCode"].(*types.AttributeValueMemberN); ok {
		var exitCode int32
		if _, err := fmt.Sscanf(v.Value, "%d", &exitCode); err == nil {
			job.ExitCode = exitCode
		}
	}
	if v, ok := item["pid"].(*types.AttributeValueMemberN); ok {
		var pid int32
		if _, err := fmt.Sscanf(v.Value, "%d", &pid); err == nil {
			job.Pid = pid
		}
	}

	// Parse infrastructure fields
	if v, ok := item["network"].(*types.AttributeValueMemberS); ok {
		job.Network = v.Value
	}
	if v, ok := item["runtime"].(*types.AttributeValueMemberS); ok {
		job.Runtime = v.Value
	}

	return job, nil
}
