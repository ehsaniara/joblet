package worker

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	pb "job-worker/api/gen"
)

//counterfeiter:generate . Service
type Service interface {
	StartJob(ctx context.Context, job *pb.Job) (*pb.Job, error)
	StopJob(jobId string) error
}
type service struct {
}

func New() Service {
	return &service{}
}

// StartJob start the job
func (s *service) StartJob(ctx context.Context, job *pb.Job) (*pb.Job, error) {
	//TODO implement me
	panic("implement me")
}

// StopJob stop the job
func (s *service) StopJob(jobId string) error {
	//TODO implement me
	panic("implement me")
}
