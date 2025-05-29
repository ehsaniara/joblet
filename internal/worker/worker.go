package worker

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"job-worker/internal/worker/task"
)

//counterfeiter:generate . Worker
type Worker interface {
	StartJob(*task.Task) (*task.Task, error)
	StopJob(jobId string) error
}
