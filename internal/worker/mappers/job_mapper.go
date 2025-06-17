package mappers

import (
	pb "job-worker/api/gen"
	"job-worker/internal/worker/domain"
)

// DomainToProtobuf converts domain Job to protobuf Job (no network fields)
func DomainToProtobuf(job *domain.Job) *pb.Job {
	pbJob := &pb.Job{
		Id:        job.Id,
		Command:   job.Command,
		Args:      job.Args,
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExitCode:  job.ExitCode,
		// Removed network fields
	}

	if job.EndTime != nil {
		pbJob.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return pbJob
}

// DomainToCreateJobResponse converts domain Job to CreateJobRes (no network fields)
func DomainToCreateJobResponse(job *domain.Job) *pb.CreateJobRes {
	response := &pb.CreateJobRes{
		Id:        job.Id,
		Command:   job.Command,
		Args:      job.Args,
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExitCode:  job.ExitCode,
		// Removed network fields
	}

	if job.EndTime != nil {
		response.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}

// DomainToGetJobResponse converts domain Job to GetJobRes (no network fields)
func DomainToGetJobResponse(job *domain.Job) *pb.GetJobRes {
	response := &pb.GetJobRes{
		Id:        job.Id,
		Command:   job.Command,
		Args:      job.Args,
		MaxCPU:    job.Limits.MaxCPU,
		MaxMemory: job.Limits.MaxMemory,
		MaxIOBPS:  job.Limits.MaxIOBPS,
		Status:    string(job.Status),
		StartTime: job.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExitCode:  job.ExitCode,
		// Removed network fields
	}

	if job.EndTime != nil {
		response.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}

// DomainToStopJobResponse converts domain Job to StopJobRes
func DomainToStopJobResponse(job *domain.Job) *pb.StopJobRes {
	response := &pb.StopJobRes{
		Id:       job.Id,
		Status:   string(job.Status),
		ExitCode: job.ExitCode,
	}

	if job.EndTime != nil {
		response.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}
