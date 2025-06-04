.PHONY: all clean job-worker job-init test

all: cli job-worker job-init

cli:
	GOOS=darwin GOARCH=amd64 go build -o bin/cli ./cmd/cli

job-worker:
	GOOS=linux GOARCH=amd64 go build -o bin/job-worker ./cmd/worker

job-init:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/job-init ./cmd/job-init

test:
	go test ./internal/...

clean:
	rm -rf bin/