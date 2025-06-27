package interfaces

import (
	"context"
	"worker/internal/worker/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Worker
type Worker interface {
	StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error)
	StopJob(ctx context.Context, jobId string) error
}
