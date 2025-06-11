package mappers

import (
	pb "job-worker/api/gen"
	"job-worker/internal/worker/domain"
	"time"
)

func formatTimePtr(t *time.Time) string {
	if t != nil {
		return t.Format(time.RFC3339)
	}
	return ""
}

func DomainToCreateJobResponse(job *domain.Job) *pb.CreateJobRes {
	return &pb.CreateJobRes{
		Id:        job.Id,
		Command:   job.Command,
		Args:      append([]string(nil), job.Args...),
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format(time.RFC3339),
		EndTime:   formatTimePtr(job.EndTime),
		ExitCode:  job.ExitCode,
	}
}

func DomainToGetJobResponse(job *domain.Job) *pb.GetJobRes {
	return &pb.GetJobRes{
		Id:        job.Id,
		Command:   job.Command,
		Args:      append([]string(nil), job.Args...),
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format(time.RFC3339),
		EndTime:   formatTimePtr(job.EndTime),
		ExitCode:  job.ExitCode,
	}
}

func DomainToStopJobResponse(job *domain.Job) *pb.StopJobRes {
	return &pb.StopJobRes{
		Id:       job.Id,
		Status:   string(job.Status),
		EndTime:  formatTimePtr(job.EndTime),
		ExitCode: job.ExitCode,
	}
}

func DomainToProtobuf(job *domain.Job) *pb.Job {
	return &pb.Job{
		Id:        job.Id,
		Command:   job.Command,
		Args:      append([]string(nil), job.Args...),
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format(time.RFC3339),
		EndTime:   formatTimePtr(job.EndTime),
		ExitCode:  job.ExitCode,
	}
}
