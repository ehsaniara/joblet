package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/state/internal/storage"
	"github.com/ehsaniara/joblet/state/internal/storage/storagefakes"
)

func TestDynamoDB_Create(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	testJob := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo test",
		Status:  domain.JobStatus("PENDING"),
		NodeId:  "node-1",
	}

	// Setup mock to succeed
	mockClient.PutItemReturns(&dynamodb.PutItemOutput{}, nil)

	err := backend.Create(context.Background(), testJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify PutItem was called
	if mockClient.PutItemCallCount() != 1 {
		t.Errorf("expected PutItem to be called once, got %d calls", mockClient.PutItemCallCount())
	}

	// Verify the call parameters
	_, input, _ := mockClient.PutItemArgsForCall(0)
	if *input.TableName != "test-table" {
		t.Errorf("expected table name 'test-table', got %s", *input.TableName)
	}

	// Verify condition expression (must not exist)
	if *input.ConditionExpression != "attribute_not_exists(jobId)" {
		t.Errorf("expected condition expression for create, got %s", *input.ConditionExpression)
	}

	// Verify job ID in item
	jobIdAttr, ok := input.Item["jobId"].(*types.AttributeValueMemberS)
	if !ok {
		t.Fatal("jobId attribute not found or wrong type")
	}
	if jobIdAttr.Value != "test-job-123" {
		t.Errorf("expected jobId 'test-job-123', got %s", jobIdAttr.Value)
	}
}

func TestDynamoDB_Create_AlreadyExists(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	testJob := &domain.Job{
		Uuid:   "duplicate-job",
		Status: domain.JobStatus("PENDING"),
	}

	// Setup mock to return ConditionalCheckFailedException
	mockClient.PutItemReturns(nil, &types.ConditionalCheckFailedException{
		Message: aws.String("The conditional request failed"),
	})

	err := backend.Create(context.Background(), testJob)
	if err != storage.ErrJobAlreadyExists {
		t.Errorf("expected ErrJobAlreadyExists, got %v", err)
	}
}

func TestDynamoDB_Get(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Setup mock to return a job
	mockItem := map[string]types.AttributeValue{
		"jobId":     &types.AttributeValueMemberS{Value: "job-123"},
		"jobStatus": &types.AttributeValueMemberS{Value: "RUNNING"},
		"command":   &types.AttributeValueMemberS{Value: "echo test"},
		"nodeId":    &types.AttributeValueMemberS{Value: "node-1"},
		"startTime": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	mockClient.GetItemReturns(&dynamodb.GetItemOutput{
		Item: mockItem,
	}, nil)

	job, err := backend.Get(context.Background(), "job-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Uuid != "job-123" {
		t.Errorf("expected job UUID 'job-123', got %s", job.Uuid)
	}
	if job.Status != domain.JobStatus("RUNNING") {
		t.Errorf("expected status RUNNING, got %s", job.Status)
	}
	if job.Command != "echo test" {
		t.Errorf("expected command 'echo test', got %s", job.Command)
	}

	// Verify GetItem was called with correct key
	if mockClient.GetItemCallCount() != 1 {
		t.Errorf("expected GetItem to be called once, got %d calls", mockClient.GetItemCallCount())
	}

	_, input, _ := mockClient.GetItemArgsForCall(0)
	keyValue, ok := input.Key["jobId"].(*types.AttributeValueMemberS)
	if !ok || keyValue.Value != "job-123" {
		t.Error("expected key with jobId='job-123'")
	}
}

func TestDynamoDB_Get_NotFound(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Setup mock to return empty result
	mockClient.GetItemReturns(&dynamodb.GetItemOutput{
		Item: nil,
	}, nil)

	_, err := backend.Get(context.Background(), "nonexistent")
	if err != storage.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestDynamoDB_Update(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	testJob := &domain.Job{
		Uuid:     "job-456",
		Status:   domain.JobStatus("COMPLETED"),
		ExitCode: 0,
	}

	mockClient.PutItemReturns(&dynamodb.PutItemOutput{}, nil)

	err := backend.Update(context.Background(), testJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify PutItem was called with existence condition
	if mockClient.PutItemCallCount() != 1 {
		t.Errorf("expected PutItem to be called once, got %d calls", mockClient.PutItemCallCount())
	}

	_, input, _ := mockClient.PutItemArgsForCall(0)
	if *input.ConditionExpression != "attribute_exists(jobId)" {
		t.Errorf("expected condition expression for update, got %s", *input.ConditionExpression)
	}
}

func TestDynamoDB_Update_NotFound(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	testJob := &domain.Job{
		Uuid:   "nonexistent",
		Status: domain.JobStatus("RUNNING"),
	}

	// Setup mock to return ConditionalCheckFailedException
	mockClient.PutItemReturns(nil, &types.ConditionalCheckFailedException{
		Message: aws.String("The conditional request failed"),
	})

	err := backend.Update(context.Background(), testJob)
	if err != storage.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestDynamoDB_Delete(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	mockClient.DeleteItemReturns(&dynamodb.DeleteItemOutput{}, nil)

	err := backend.Delete(context.Background(), "job-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify DeleteItem was called
	if mockClient.DeleteItemCallCount() != 1 {
		t.Errorf("expected DeleteItem to be called once, got %d calls", mockClient.DeleteItemCallCount())
	}

	_, input, _ := mockClient.DeleteItemArgsForCall(0)
	keyValue, ok := input.Key["jobId"].(*types.AttributeValueMemberS)
	if !ok || keyValue.Value != "job-to-delete" {
		t.Error("expected key with jobId='job-to-delete'")
	}
}

func TestDynamoDB_List(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Setup mock to return multiple jobs
	mockItems := []map[string]types.AttributeValue{
		{
			"jobId":     &types.AttributeValueMemberS{Value: "job-1"},
			"jobStatus": &types.AttributeValueMemberS{Value: "RUNNING"},
			"command":   &types.AttributeValueMemberS{Value: "echo 1"},
			"nodeId":    &types.AttributeValueMemberS{Value: "node-1"},
		},
		{
			"jobId":     &types.AttributeValueMemberS{Value: "job-2"},
			"jobStatus": &types.AttributeValueMemberS{Value: "RUNNING"},
			"command":   &types.AttributeValueMemberS{Value: "echo 2"},
			"nodeId":    &types.AttributeValueMemberS{Value: "node-1"},
		},
	}

	mockClient.ScanReturns(&dynamodb.ScanOutput{
		Items: mockItems,
		Count: 2,
	}, nil)

	jobs, err := backend.List(context.Background(), &storage.Filter{
		Status: "RUNNING",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	// Verify Scan was called with filter
	if mockClient.ScanCallCount() != 1 {
		t.Errorf("expected Scan to be called once, got %d calls", mockClient.ScanCallCount())
	}

	_, input, _ := mockClient.ScanArgsForCall(0)

	// Verify filter expression
	if input.FilterExpression == nil || *input.FilterExpression != "jobStatus = :status" {
		t.Error("expected filter expression for status")
	}

	// Verify limit
	if input.Limit == nil || *input.Limit != 10 {
		t.Error("expected limit to be set to 10")
	}

	// Verify expression attribute values
	statusValue, ok := input.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS)
	if !ok || statusValue.Value != "RUNNING" {
		t.Error("expected status filter value 'RUNNING'")
	}
}

func TestDynamoDB_List_NoFilter(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	mockClient.ScanReturns(&dynamodb.ScanOutput{
		Items: []map[string]types.AttributeValue{},
		Count: 0,
	}, nil)

	jobs, err := backend.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}

	// Verify Scan was called without filter
	_, input, _ := mockClient.ScanArgsForCall(0)
	if input.FilterExpression != nil {
		t.Error("expected no filter expression when filter is nil")
	}
}

func TestDynamoDB_Sync(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Create test jobs
	jobs := []*domain.Job{
		{Uuid: "sync-job-1", Status: domain.JobStatus("PENDING")},
		{Uuid: "sync-job-2", Status: domain.JobStatus("RUNNING")},
		{Uuid: "sync-job-3", Status: domain.JobStatus("COMPLETED")},
	}

	mockClient.BatchWriteItemReturns(&dynamodb.BatchWriteItemOutput{}, nil)

	err := backend.Sync(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify BatchWriteItem was called once (3 jobs < 25 batch size)
	if mockClient.BatchWriteItemCallCount() != 1 {
		t.Errorf("expected BatchWriteItem to be called once, got %d calls", mockClient.BatchWriteItemCallCount())
	}

	_, input, _ := mockClient.BatchWriteItemArgsForCall(0)
	writeRequests := input.RequestItems["test-table"]
	if len(writeRequests) != 3 {
		t.Errorf("expected 3 write requests, got %d", len(writeRequests))
	}
}

func TestDynamoDB_Sync_LargeBatch(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Create 50 jobs (should be split into 2 batches of 25 each)
	jobs := make([]*domain.Job, 50)
	for i := 0; i < 50; i++ {
		jobs[i] = &domain.Job{
			Uuid:   fmt.Sprintf("job-%d", i),
			Status: domain.JobStatus("PENDING"),
		}
	}

	mockClient.BatchWriteItemReturns(&dynamodb.BatchWriteItemOutput{}, nil)

	err := backend.Sync(context.Background(), jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify BatchWriteItem was called twice (50 jobs / 25 batch size)
	if mockClient.BatchWriteItemCallCount() != 2 {
		t.Errorf("expected BatchWriteItem to be called twice, got %d calls", mockClient.BatchWriteItemCallCount())
	}

	// Verify first batch has 25 items
	_, input1, _ := mockClient.BatchWriteItemArgsForCall(0)
	if len(input1.RequestItems["test-table"]) != 25 {
		t.Errorf("expected 25 items in first batch, got %d", len(input1.RequestItems["test-table"]))
	}

	// Verify second batch has 25 items
	_, input2, _ := mockClient.BatchWriteItemArgsForCall(1)
	if len(input2.RequestItems["test-table"]) != 25 {
		t.Errorf("expected 25 items in second batch, got %d", len(input2.RequestItems["test-table"]))
	}
}

func TestDynamoDB_HealthCheck(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	mockClient.DescribeTableReturns(&dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:   aws.String("test-table"),
			TableStatus: types.TableStatusActive,
		},
	}, nil)

	err := backend.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify DescribeTable was called
	if mockClient.DescribeTableCallCount() != 1 {
		t.Errorf("expected DescribeTable to be called once, got %d calls", mockClient.DescribeTableCallCount())
	}
}

func TestDynamoDB_HealthCheck_TableNotFound(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "nonexistent-table", 30)

	mockClient.DescribeTableReturns(nil, &types.ResourceNotFoundException{
		Message: aws.String("Table not found"),
	})

	err := backend.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for nonexistent table")
	}

	storageErr, ok := err.(*storage.StorageError)
	if !ok {
		t.Errorf("expected StorageError, got %T", err)
	}
	if storageErr.Code != "TABLE_NOT_FOUND" {
		t.Errorf("expected error code TABLE_NOT_FOUND, got %s", storageErr.Code)
	}
}

func TestDynamoDB_Close(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	err := backend.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
