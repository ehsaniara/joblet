package store

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	pb "job-worker/api/gen"
	"job-worker/internal/worker/task"
	"sync"
)

//counterfeiter:generate . Store
type Store interface {
	GetJob(id string) (*pb.Job, bool)
	GetJobs() []*pb.Job
}

type store struct {
	tasks map[string]*task.Task
	mutex sync.RWMutex
}

func New() Store {
	return &store{
		tasks: make(map[string]*task.Task),
	}
}

func (s *store) GetJob(id string) (*pb.Job, bool) {
	//TODO implement me
	panic("implement me")
}

func (s *store) GetJobs() []*pb.Job {
	//TODO implement me
	panic("implement me")
}
