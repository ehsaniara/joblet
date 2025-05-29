package task

import (
	pb "job-worker/api/gen"
	"sync"
)

type Task struct {
	*pb.Job
	Mu     sync.Mutex
	Buffer [][]byte
}
