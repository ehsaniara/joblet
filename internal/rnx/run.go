package rnx

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pb "joblet/api/gen"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a new job",
		Long: `Run a new job with the specified command and arguments.

Examples:
  rnx run nginx
  rnx run python3 script.py
  rnx run bash -c "curl https://example.com"
  rnx --node=srv1 run ps aux

File Upload Examples:
  rnx run --upload=script.py python3 script.py
  rnx run --upload-dir=. python3 main.py
  rnx run --upload=data.csv --upload=process.py python3 process.py data.csv

Flags:
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS
  --upload=FILE       Upload a file to the job workspace
  --upload-dir=DIR    Upload entire directory to the job workspace`,
		Args:               cobra.MinimumNArgs(1),
		RunE:               runRun,
		DisableFlagParsing: true,
	}

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	var (
		maxCPU     int32
		cpuCores   string
		maxMemory  int32
		maxIOBPS   int32
		uploads    []string
		uploadDirs []string
	)

	commandStartIndex := 0
	for i, arg := range args {
		if strings.HasPrefix(arg, "--cpu-cores=") {
			cpuCores = strings.TrimPrefix(arg, "--cpu-cores=")
		} else if strings.HasPrefix(arg, "--max-cpu=") {
			if val, err := parseIntFlag(arg, "--max-cpu="); err == nil {
				maxCPU = int32(val)
			}
		} else if strings.HasPrefix(arg, "--max-memory=") {
			if val, err := parseIntFlag(arg, "--max-memory="); err == nil {
				maxMemory = int32(val)
			}
		} else if strings.HasPrefix(arg, "--max-iobps=") {
			if val, err := parseIntFlag(arg, "--max-iobps="); err == nil {
				maxIOBPS = int32(val)
			}
		} else if strings.HasPrefix(arg, "--upload=") {
			uploadPath := strings.TrimPrefix(arg, "--upload=")
			uploads = append(uploads, uploadPath)
		} else if strings.HasPrefix(arg, "--upload-dir=") {
			uploadDir := strings.TrimPrefix(arg, "--upload-dir=")
			uploadDirs = append(uploadDirs, uploadDir)
		} else if !strings.HasPrefix(arg, "--") {
			commandStartIndex = i
			break
		} else {
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if commandStartIndex >= len(args) {
		return fmt.Errorf("must specify a command")
	}

	commandArgs := args[commandStartIndex:]
	command := commandArgs[0]
	cmdArgs := commandArgs[1:]

	// client creation using unified config
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var fileUploads []*pb.FileUpload

	// individual file uploads
	for _, uploadPath := range uploads {
		files, err := collectFileUploads(uploadPath, false)
		if err != nil {
			return fmt.Errorf("failed to prepare upload %s: %w", uploadPath, err)
		}
		fileUploads = append(fileUploads, files...)
	}

	// directory uploads
	for _, uploadDir := range uploadDirs {
		files, err := collectFileUploads(uploadDir, true)
		if err != nil {
			return fmt.Errorf("failed to prepare upload directory %s: %w", uploadDir, err)
		}
		fileUploads = append(fileUploads, files...)
	}

	// show upload summary if files are being uploaded
	if len(fileUploads) > 0 {
		totalSize := int64(0)
		for _, f := range fileUploads {
			totalSize += int64(len(f.Content))
		}
		fmt.Printf("Uploading %d files (%.2f MB)...\n", len(fileUploads), float64(totalSize)/1024/1024)
	}

	job := &pb.RunJobReq{
		Command:   command,
		Args:      cmdArgs,
		MaxCPU:    maxCPU,
		CpuCores:  cpuCores,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
		Uploads:   fileUploads,
	}

	response, err := jobClient.RunJob(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to run job: %v", err)
	}

	fmt.Printf("Job started:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Command: %s\n", strings.Join(commandArgs, " "))
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("StartTime: %s\n", response.StartTime)
	if len(fileUploads) > 0 {
		fmt.Printf("Uploaded: %d files\n", len(fileUploads))
	}

	return nil
}

func collectFileUploads(path string, isDir bool) ([]*pb.FileUpload, error) {
	var uploads []*pb.FileUpload

	// Get absolute path for proper relative path calculation
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	// If it's a file and we expected a directory, error
	if isDir && !info.IsDir() {
		return nil, fmt.Errorf("expected directory but got file: %s", path)
	}

	// If it's a directory and we didn't expect one, error
	if !isDir && info.IsDir() {
		return nil, fmt.Errorf("expected file but got directory: %s (use --upload-dir for directories)", path)
	}

	if info.IsDir() {
		// Walk the directory tree
		var baseDir string

		// the base should be the parent of the directory
		// so that the directory name itself is preserved in the relative path
		if isDir {
			baseDir = filepath.Dir(absPath)
		} else {
			// when current directory uploads (path == ".")
			baseDir = absPath
		}

		err = filepath.Walk(absPath, func(filePath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate relative path
			relPath, err := filepath.Rel(baseDir, filePath)
			if err != nil {
				return fmt.Errorf("failed to calculate relative path: %w", err)
			}

			// Convert to forward slashes for consistency
			relPath = filepath.ToSlash(relPath)

			if fileInfo.IsDir() {
				// Add directory entry
				uploads = append(uploads, &pb.FileUpload{
					Path:        relPath,
					Content:     nil,
					Mode:        uint32(fileInfo.Mode().Perm()),
					IsDirectory: true,
				})
			} else {
				// Read file content
				content, e := os.ReadFile(filePath)
				if e != nil {
					return fmt.Errorf("failed to read file %s: %w", filePath, e)
				}

				// Skip very large files
				if len(content) > 50*1024*1024 { // 50MB limit per file
					fmt.Printf("Warning: Skipping large file %s (%.2f MB)\n", relPath, float64(len(content))/1024/1024)
					return nil
				}

				uploads = append(uploads, &pb.FileUpload{
					Path:        relPath,
					Content:     content,
					Mode:        uint32(fileInfo.Mode().Perm()),
					IsDirectory: false,
				})
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Single file upload
		content, err := io.ReadAll(io.LimitReader(openFile(absPath), 50*1024*1024)) // 50MB limit
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		uploads = append(uploads, &pb.FileUpload{
			Path:        filepath.Base(absPath),
			Content:     content,
			Mode:        uint32(info.Mode().Perm()),
			IsDirectory: false,
		})
	}

	return uploads, nil
}

func openFile(path string) io.ReadCloser {
	file, err := os.Open(path)
	if err != nil {
		return io.NopCloser(strings.NewReader(""))
	}
	return file
}

func parseIntFlag(arg, prefix string) (int64, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.ParseInt(valueStr, 10, 32)
}
