package domain

// UploadStreamer handles streaming file uploads through pipes
type UploadStreamer interface {
	StartStreaming() error
	SetManager(manager UploadManager)
	GetPipePath() string
	GetJobID() string
}

// UploadManager handles upload operations
type UploadManager interface {
	PrepareUploadSession(jobID string, uploads []FileUpload, memoryLimitMB int32) (*UploadSession, error)
	CreateUploadPipe(jobID string) (string, error)
	CleanupPipe(pipePath string) error
}

// UploadSessionFactory creates upload streaming sessions
type UploadSessionFactory interface {
	CreateStreamContext(session *UploadSession, pipePath string, jobID string) UploadStreamer
}
