package domain

// FileUpload represents a file to be uploaded to the job workspace
type FileUpload struct {
	Path        string // Relative path in job workspace
	Content     []byte // File content
	Mode        uint32 // Unix file permissions
	IsDirectory bool   // True if this represents a directory
}
