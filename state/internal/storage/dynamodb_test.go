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
	"github.com/ehsaniara/joblet/internal/joblet/domain/values"
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
	jobIdAttr, ok := input.Item["job_uuid"].(*types.AttributeValueMemberS)
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
		"job_uuid":  &types.AttributeValueMemberS{Value: "job-123"},
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
	keyValue, ok := input.Key["job_uuid"].(*types.AttributeValueMemberS)
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
	keyValue, ok := input.Key["job_uuid"].(*types.AttributeValueMemberS)
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
			"job_uuid":  &types.AttributeValueMemberS{Value: "job-1"},
			"jobStatus": &types.AttributeValueMemberS{Value: "RUNNING"},
			"command":   &types.AttributeValueMemberS{Value: "echo 1"},
			"nodeId":    &types.AttributeValueMemberS{Value: "node-1"},
		},
		{
			"job_uuid":  &types.AttributeValueMemberS{Value: "job-2"},
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

// TestDynamoDB_AllFieldsStored verifies that all job fields are stored in DynamoDB
func TestDynamoDB_AllFieldsStored(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Create a job with all fields populated
	cpuLimit, _ := values.NewCPUPercentage(50)
	memLimit, _ := values.NewMemorySize(1024 * 1024 * 100) // 100MB
	ioLimit, _ := values.NewBandwidth(1024 * 1024)         // 1MB/s
	cpuCores, _ := values.ParseCPUCoreSet("0-3")

	testJob := &domain.Job{
		Uuid:    "full-job-123",
		Command: "python",
		Args:    []string{"script.py", "--verbose", "--output=/tmp/out"},
		Type:    domain.JobType("standard"),
		Status:  domain.JobStatus("RUNNING"),
		NodeId:  "node-1",
		Limits: domain.ResourceLimits{
			CPU:         cpuLimit,
			CPUCores:    cpuCores,
			Memory:      memLimit,
			IOBandwidth: ioLimit,
		},
		Network:          "custom-network",
		Volumes:          []string{"vol1", "vol2"},
		Runtime:          "python:3.9",
		WorkingDirectory: "/app/workdir",
		Environment:      map[string]string{"ENV1": "value1", "ENV2": "value2"},
		GPUIndices:       []int32{0, 1},
		GPUCount:         2,
		GPUMemoryMB:      4096,
		Pid:              12345,
		ExitCode:         0,
	}

	mockClient.PutItemReturns(&dynamodb.PutItemOutput{}, nil)

	err := backend.Create(context.Background(), testJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the stored item
	_, input, _ := mockClient.PutItemArgsForCall(0)
	item := input.Item

	// Verify all fields are present
	tests := []struct {
		field    string
		expected interface{}
	}{
		{"job_uuid", "full-job-123"},
		{"command", "python"},
		{"jobStatus", "RUNNING"},
		{"nodeId", "node-1"},
		{"network", "custom-network"},
		{"runtime", "python:3.9"},
		{"workingDirectory", "/app/workdir"},
	}

	for _, tt := range tests {
		attr, ok := item[tt.field].(*types.AttributeValueMemberS)
		if !ok {
			t.Errorf("field %s not found or wrong type", tt.field)
			continue
		}
		if attr.Value != tt.expected {
			t.Errorf("field %s: expected %v, got %v", tt.field, tt.expected, attr.Value)
		}
	}

	// Verify args list
	argsAttr, ok := item["args"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatal("args field not found or wrong type")
	}
	if len(argsAttr.Value) != 3 {
		t.Errorf("expected 3 args, got %d", len(argsAttr.Value))
	}

	// Verify volumes list
	volAttr, ok := item["volumes"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatal("volumes field not found or wrong type")
	}
	if len(volAttr.Value) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(volAttr.Value))
	}

	// Verify environment map
	envAttr, ok := item["environment"].(*types.AttributeValueMemberM)
	if !ok {
		t.Fatal("environment field not found or wrong type")
	}
	if len(envAttr.Value) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(envAttr.Value))
	}

	// Verify limits map
	limitsAttr, ok := item["limits"].(*types.AttributeValueMemberM)
	if !ok {
		t.Fatal("limits field not found or wrong type")
	}
	if _, ok := limitsAttr.Value["cpu"]; !ok {
		t.Error("limits.cpu not found")
	}
	if _, ok := limitsAttr.Value["memory"]; !ok {
		t.Error("limits.memory not found")
	}

	// Verify GPU fields
	gpuIndicesAttr, ok := item["gpuIndices"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatal("gpuIndices field not found or wrong type")
	}
	if len(gpuIndicesAttr.Value) != 2 {
		t.Errorf("expected 2 GPU indices, got %d", len(gpuIndicesAttr.Value))
	}

	gpuCountAttr, ok := item["gpuCount"].(*types.AttributeValueMemberN)
	if !ok {
		t.Fatal("gpuCount field not found or wrong type")
	}
	if gpuCountAttr.Value != "2" {
		t.Errorf("expected gpuCount '2', got %s", gpuCountAttr.Value)
	}

	gpuMemAttr, ok := item["gpuMemoryMB"].(*types.AttributeValueMemberN)
	if !ok {
		t.Fatal("gpuMemoryMB field not found or wrong type")
	}
	if gpuMemAttr.Value != "4096" {
		t.Errorf("expected gpuMemoryMB '4096', got %s", gpuMemAttr.Value)
	}
}

// TestDynamoDB_AllFieldsRetrieved verifies that all job fields are correctly retrieved from DynamoDB
func TestDynamoDB_AllFieldsRetrieved(t *testing.T) {
	mockClient := &storagefakes.FakeDynamoDBAPI{}
	backend := storage.NewDynamoDBBackendWithClient(mockClient, "test-table", 30)

	// Create a mock DynamoDB item with all fields
	mockItem := map[string]types.AttributeValue{
		"job_uuid":         &types.AttributeValueMemberS{Value: "full-job-456"},
		"command":          &types.AttributeValueMemberS{Value: "node"},
		"jobStatus":        &types.AttributeValueMemberS{Value: "COMPLETED"},
		"nodeId":           &types.AttributeValueMemberS{Value: "node-2"},
		"jobType":          &types.AttributeValueMemberS{Value: "standard"},
		"network":          &types.AttributeValueMemberS{Value: "bridge"},
		"runtime":          &types.AttributeValueMemberS{Value: "node:18"},
		"workingDirectory": &types.AttributeValueMemberS{Value: "/work"},
		"startTime":        &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"exitCode":         &types.AttributeValueMemberN{Value: "0"},
		"pid":              &types.AttributeValueMemberN{Value: "9999"},
		"args": &types.AttributeValueMemberL{Value: []types.AttributeValue{
			&types.AttributeValueMemberS{Value: "app.js"},
			&types.AttributeValueMemberS{Value: "--port=3000"},
		}},
		"volumes": &types.AttributeValueMemberL{Value: []types.AttributeValue{
			&types.AttributeValueMemberS{Value: "data-vol"},
		}},
		"environment": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"NODE_ENV": &types.AttributeValueMemberS{Value: "production"},
		}},
		"limits": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"cpu":         &types.AttributeValueMemberN{Value: "75"},
			"memory":      &types.AttributeValueMemberN{Value: "536870912"}, // 512MB
			"ioBandwidth": &types.AttributeValueMemberN{Value: "10485760"},  // 10MB/s
			"cpuCores":    &types.AttributeValueMemberS{Value: "0-1"},
		}},
		"gpuIndices": &types.AttributeValueMemberL{Value: []types.AttributeValue{
			&types.AttributeValueMemberN{Value: "0"},
		}},
		"gpuCount":    &types.AttributeValueMemberN{Value: "1"},
		"gpuMemoryMB": &types.AttributeValueMemberN{Value: "8192"},
	}

	mockClient.GetItemReturns(&dynamodb.GetItemOutput{Item: mockItem}, nil)

	job, err := backend.Get(context.Background(), "full-job-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields are correctly parsed
	if job.Uuid != "full-job-456" {
		t.Errorf("expected UUID 'full-job-456', got %s", job.Uuid)
	}
	if job.Command != "node" {
		t.Errorf("expected Command 'node', got %s", job.Command)
	}
	if len(job.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(job.Args))
	}
	if job.Args[0] != "app.js" {
		t.Errorf("expected first arg 'app.js', got %s", job.Args[0])
	}
	if job.Type != domain.JobType("standard") {
		t.Errorf("expected Type 'standard', got %s", job.Type)
	}
	if job.Status != domain.JobStatus("COMPLETED") {
		t.Errorf("expected Status 'COMPLETED', got %s", job.Status)
	}
	if job.Network != "bridge" {
		t.Errorf("expected Network 'bridge', got %s", job.Network)
	}
	if job.Runtime != "node:18" {
		t.Errorf("expected Runtime 'node:18', got %s", job.Runtime)
	}
	if len(job.Volumes) != 1 || job.Volumes[0] != "data-vol" {
		t.Errorf("expected Volumes ['data-vol'], got %v", job.Volumes)
	}
	if job.WorkingDirectory != "/work" {
		t.Errorf("expected WorkingDirectory '/work', got %s", job.WorkingDirectory)
	}
	if job.Environment["NODE_ENV"] != "production" {
		t.Errorf("expected Environment['NODE_ENV']='production', got %s", job.Environment["NODE_ENV"])
	}

	// Verify limits
	if job.Limits.CPU.Value() != 75 {
		t.Errorf("expected CPU limit 75, got %d", job.Limits.CPU.Value())
	}
	if job.Limits.Memory.Bytes() != 536870912 {
		t.Errorf("expected Memory limit 536870912, got %d", job.Limits.Memory.Bytes())
	}
	if job.Limits.IOBandwidth.BytesPerSecond() != 10485760 {
		t.Errorf("expected IO limit 10485760, got %d", job.Limits.IOBandwidth.BytesPerSecond())
	}

	// Verify GPU fields
	if len(job.GPUIndices) != 1 || job.GPUIndices[0] != 0 {
		t.Errorf("expected GPUIndices [0], got %v", job.GPUIndices)
	}
	if job.GPUCount != 1 {
		t.Errorf("expected GPUCount 1, got %d", job.GPUCount)
	}
	if job.GPUMemoryMB != 8192 {
		t.Errorf("expected GPUMemoryMB 8192, got %d", job.GPUMemoryMB)
	}
	if job.Pid != 9999 {
		t.Errorf("expected Pid 9999, got %d", job.Pid)
	}
	if job.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", job.ExitCode)
	}
}
