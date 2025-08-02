package domain

import (
	"fmt"
	"io"
	"os"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// UploadTransport abstracts the transport mechanism for uploads
//
//counterfeiter:generate . UploadTransport
type UploadTransport interface {
	// GetWriter returns a writer for the upload data
	GetWriter() (io.WriteCloser, error)

	// GetReader returns a reader for receiving upload data
	GetReader() (io.ReadCloser, error)

	// Close cleans up transport resources
	Close() error
}

// UploadStreamer handles streaming file uploads without exposing implementation
//
//counterfeiter:generate . UploadStreamer
type UploadStreamer interface {
	// Start begins the streaming process
	Start() error

	// Stop gracefully stops the streaming
	Stop() error

	// SetManager sets the upload manager
	SetManager(manager UploadManager)

	// GetJobID returns the job ID this streamer is associated with
	GetJobID() string

	// GetTransport returns the transport mechanism
	GetTransport() UploadTransport
}

// UploadManager handles upload operations with better abstraction
//
//counterfeiter:generate . UploadManager
type UploadManager interface {
	// PrepareUploadSession prepares an upload session
	PrepareUploadSession(jobID string, uploads []FileUpload, memoryLimitMB int32) (*UploadSession, error)

	// CreateTransport creates a transport for uploads (replaces CreateUploadPipe)
	CreateTransport(jobID string) (UploadTransport, error)

	// CleanupTransport cleans up transport resources
	CleanupTransport(transport UploadTransport) error
}

// PipeTransport implements UploadTransport using named pipes
type PipeTransport struct {
	pipePath string
	platform platform.Platform
	logger   *logger.Logger
	writer   io.WriteCloser
	reader   io.ReadCloser
}

// NewPipeTransport creates a new pipe transport
func NewPipeTransport(pipePath string, platform platform.Platform, logger *logger.Logger) *PipeTransport {
	return &PipeTransport{
		pipePath: pipePath,
		platform: platform,
		logger:   logger.WithField("component", "pipe-transport"),
	}
}

// GetPath returns the pipe file path
func (p *PipeTransport) GetPath() string {
	return p.pipePath
}

func (p *PipeTransport) GetWriter() (io.WriteCloser, error) {
	if p.writer != nil {
		return p.writer, nil
	}

	writer, err := p.platform.OpenFile(p.pipePath, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open pipe for writing: %w", err)
	}

	p.writer = writer
	p.logger.Debug("pipe writer opened", "path", p.pipePath)
	return writer, nil
}

func (p *PipeTransport) GetReader() (io.ReadCloser, error) {
	if p.reader != nil {
		return p.reader, nil
	}

	reader, err := p.platform.OpenFile(p.pipePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open pipe for reading: %w", err)
	}

	p.reader = reader
	p.logger.Debug("pipe reader opened", "path", p.pipePath)
	return reader, nil
}

func (p *PipeTransport) Close() error {
	var errs []error

	if p.writer != nil {
		if err := p.writer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close writer: %w", err))
		}
		p.writer = nil
	}

	if p.reader != nil {
		if err := p.reader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close reader: %w", err))
		}
		p.reader = nil
	}

	// Clean up the pipe file
	if err := p.platform.Remove(p.pipePath); err != nil && !p.platform.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove pipe: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("transport cleanup errors: %v", errs)
	}

	p.logger.Debug("pipe transport closed", "path", p.pipePath)
	return nil
}
