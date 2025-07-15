package mappers

import (
	pb "joblet/api/gen"
	"joblet/internal/joblet/domain"
)

// DomainToProtobuf converts domain Job to protobuf Job
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

// DomainToRunJobResponse converts domain Job to RunJobRes
func DomainToRunJobResponse(job *domain.Job) *pb.RunJobRes {
	response := &pb.RunJobRes{
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

// DomainToGetJobStatusResponse converts domain Job to GetJobStatusRes
func DomainToGetJobStatusResponse(job *domain.Job) *pb.GetJobStatusRes {
	response := &pb.GetJobStatusRes{
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

// ProtobufToFileUpload converts protobuf FileUpload to domain FileUpload
func ProtobufToFileUpload(uploads []*pb.FileUpload) []domain.FileUpload {
	domainUploads := make([]domain.FileUpload, 0, len(uploads))

	for _, pbUpload := range uploads {
		domainUploads = append(domainUploads, domain.FileUpload{
			Path:        pbUpload.Path,
			Content:     pbUpload.Content,
			Mode:        pbUpload.Mode,
			IsDirectory: pbUpload.IsDirectory,
		})
	}

	return domainUploads
}
