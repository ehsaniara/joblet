package task

import (
	pb "job-worker/api/gen"
	"job-worker/config"
	"sync"
)

type Task struct {
	*pb.Job

	Mu     sync.Mutex
	Buffer [][]byte
}

func NewTask(job *pb.Job) *Task {
	t := &Task{
		Job:    job,
		Buffer: make([][]byte, 0, config.MaxLogBufferSize),
	}

	return t
}
