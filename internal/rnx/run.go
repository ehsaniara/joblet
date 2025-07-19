package rnx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pb "joblet/api/gen"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a new job immediately or schedule it for later",
		Long: `Run a new job with the specified command and arguments, either immediately or scheduled for future execution.

Examples:
  # Immediate execution
  rnx run nginx
  rnx run python3 script.py
  rnx run bash -c "curl https://example.com"
  rnx --node=srv1 run ps aux

  # Scheduled execution
  rnx run --schedule="1hour" python3 script.py
  rnx run --schedule="30min" echo "Hello World"
  rnx run --schedule="2025-07-18T20:02:48" backup_script.sh
  rnx run --schedule="2h30m" --max-memory=512 data_processing.py

File Upload Examples:
  # Uploads work with both immediate and scheduled jobs
  rnx run --upload=script.py python3 script.py
  rnx run --schedule="1hour" --upload-dir=. python3 main.py
  rnx run --schedule="30min" --upload=data.csv --upload=process.py python3 process.py

Scheduling Formats:
  # Relative time
  --schedule="1hour"      # 1 hour from now
  --schedule="30min"      # 30 minutes from now
  --schedule="2h30m"      # 2 hours 30 minutes from now
  --schedule="45s"        # 45 seconds from now

  # Absolute time (RFC3339 format)
  --schedule="2025-07-18T20:02:48"           # Local time
  --schedule="2025-07-18T20:02:48Z"          # UTC time
  --schedule="2025-07-18T20:02:48-07:00"     # With timezone

Flags:
  --schedule=SPEC     Schedule job for future execution
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS
  --cpu-cores=SPEC    CPU cores specification
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
		schedule   string // New: schedule specification
	)

	commandStartIndex := 0
	for i, arg := range args {
		if strings.HasPrefix(arg, "--schedule=") {
			schedule = strings.TrimPrefix(arg, "--schedule=")
		} else if strings.HasPrefix(arg, "--cpu-cores=") {
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

	// Client creation using unified config
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Process file uploads
	fileUploads, err := processFileUploads(uploads, uploadDirs)
	if err != nil {
		return fmt.Errorf("file upload processing failed: %w", err)
	}

	// Display upload summary if files are being uploaded
	if len(fileUploads) > 0 {
		totalSize := int64(0)
		for _, upload := range fileUploads {
			totalSize += int64(len(upload.Content))
		}

		fmt.Printf("📤 Uploading %d files (%.2f MB)...\n",
			len(fileUploads), float64(totalSize)/1024/1024)
	}

	// Process schedule on client side
	var scheduledTimeRFC3339 string
	if schedule != "" {
		scheduledTime, err := parseScheduleOnClient(schedule)
		if err != nil {
			return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
		}

		// Convert to RFC3339 format for server
		scheduledTimeRFC3339 = scheduledTime.Format("2006-01-02T15:04:05Z07:00")
	}

	// Create job request with RFC3339 formatted schedule
	request := &pb.RunJobReq{
		Command:   command,
		Args:      cmdArgs,
		MaxCPU:    maxCPU,
		CpuCores:  cpuCores,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
		Uploads:   fileUploads,
		Schedule:  scheduledTimeRFC3339, // Send RFC3339 string to server
	}

	// Submit job
	response, err := jobClient.RunJob(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to run job: %v", err)
	}

	fmt.Printf("Job started:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Command: %s %s\n", response.Command, strings.Join(response.Args, " "))
	fmt.Printf("Status: %s\n", response.Status)
	if schedule != "" {
		fmt.Printf("Schedule Input: %s\n", schedule) // Show user's original input
		fmt.Printf("Scheduled Time: %s\n", response.ScheduledTime)
	} else {
		fmt.Printf("StartTime: %s\n", response.StartTime)
	}

	if len(fileUploads) > 0 {
		fmt.Printf("Files: %d uploaded successfully\n", len(fileUploads))
	}

	return nil
}

func parseIntFlag(arg, prefix string) (int, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.Atoi(valueStr)
}

func processFileUploads(uploads []string, uploadDirs []string) ([]*pb.FileUpload, error) {
	var result []*pb.FileUpload

	// Process individual file uploads
	for _, uploadPath := range uploads {
		fileInfo, err := os.Stat(uploadPath)
		if err != nil {
			return nil, fmt.Errorf("cannot access upload file %s: %w", uploadPath, err)
		}

		if fileInfo.IsDir() {
			return nil, fmt.Errorf("use --upload-dir for directories: %s", uploadPath)
		}

		content, err := os.ReadFile(uploadPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read upload file %s: %w", uploadPath, err)
		}

		result = append(result, &pb.FileUpload{
			Path:        filepath.Base(uploadPath),
			Content:     content,
			Mode:        uint32(fileInfo.Mode()),
			IsDirectory: false,
		})
	}

	// Process directory uploads
	for _, uploadDir := range uploadDirs {
		dirUploads, err := processDirectoryUpload(uploadDir)
		if err != nil {
			return nil, fmt.Errorf("directory upload failed for %s: %w", uploadDir, err)
		}
		result = append(result, dirUploads...)
	}

	return result, nil
}

func processDirectoryUpload(dir string) ([]*pb.FileUpload, error) {
	var uploads []*pb.FileUpload

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			// Create directory entry
			uploads = append(uploads, &pb.FileUpload{
				Path:        relPath,
				Content:     nil,
				Mode:        uint32(info.Mode()),
				IsDirectory: true,
			})
		} else {
			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("cannot read file %s: %w", path, err)
			}

			uploads = append(uploads, &pb.FileUpload{
				Path:        relPath,
				Content:     content,
				Mode:        uint32(info.Mode()),
				IsDirectory: false,
			})
		}

		return nil
	})

	return uploads, err
}

// parseScheduleOnClient parses schedule specifications on the client side
func parseScheduleOnClient(scheduleSpec string) (time.Time, error) {
	if scheduleSpec == "" {
		return time.Time{}, fmt.Errorf("schedule specification cannot be empty")
	}

	// Try parsing as absolute time first (RFC3339 format)
	if absoluteTime, err := parseAbsoluteTime(scheduleSpec); err == nil {
		return absoluteTime, nil
	}

	// Try parsing as relative time
	if relativeTime, err := parseRelativeTime(scheduleSpec); err == nil {
		return relativeTime, nil
	}

	return time.Time{}, fmt.Errorf("invalid format. Examples: '1min', '30min', '1hour', '2h30m', '45s' or '2025-07-18T20:02:48'")
}

// parseAbsoluteTime parses absolute time specifications
func parseAbsoluteTime(spec string) (time.Time, error) {
	// Common time formats to try
	formats := []string{
		time.RFC3339,          // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,      // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05", // Without timezone
		"2006-01-02 15:04:05", // Space instead of T
		"2006-01-02T15:04",    // Without seconds
		"2006-01-02 15:04",    // Space, no seconds
	}

	for _, format := range formats {
		if t, err := time.Parse(format, spec); err == nil {
			// If no timezone specified, assume local time
			if t.Location() == time.UTC && !strings.Contains(spec, "Z") && !strings.Contains(spec, "+") && !strings.Contains(spec, "-") {
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid absolute time format: %s", spec)
}

// parseRelativeTime parses relative time specifications
func parseRelativeTime(spec string) (time.Time, error) {
	// Normalize the input - remove spaces and convert to lowercase
	spec = strings.ToLower(strings.ReplaceAll(spec, " ", ""))

	// Handle common shorthand cases first
	switch spec {
	case "1min", "1m":
		return time.Now().Add(1 * time.Minute), nil
	case "5min", "5m":
		return time.Now().Add(5 * time.Minute), nil
	case "10min", "10m":
		return time.Now().Add(10 * time.Minute), nil
	case "30min", "30m":
		return time.Now().Add(30 * time.Minute), nil
	case "1hour", "1h":
		return time.Now().Add(1 * time.Hour), nil
	case "2hour", "2h":
		return time.Now().Add(2 * time.Hour), nil
	}

	// Regular expression to match time components
	re := regexp.MustCompile(`(\d+)\s*(h|hour|hours|m|min|mins|minute|minutes|s|sec|secs|second|seconds)\b`)
	matches := re.FindAllStringSubmatch(spec, -1)

	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("no valid time components found")
	}

	var totalDuration time.Duration

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		value, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number: %s", match[1])
		}

		unit := strings.TrimSpace(match[2])
		var duration time.Duration

		switch unit {
		case "h", "hour", "hours":
			duration = time.Duration(value) * time.Hour
		case "m", "min", "mins", "minute", "minutes":
			duration = time.Duration(value) * time.Minute
		case "s", "sec", "secs", "second", "seconds":
			duration = time.Duration(value) * time.Second
		default:
			return time.Time{}, fmt.Errorf("unknown time unit: %s", unit)
		}

		totalDuration += duration
	}

	if totalDuration == 0 {
		return time.Time{}, fmt.Errorf("total duration cannot be zero")
	}

	// Validate duration bounds
	if totalDuration < time.Second {
		return time.Time{}, fmt.Errorf("duration too short (minimum 1 second)")
	}

	if totalDuration > 365*24*time.Hour {
		return time.Time{}, fmt.Errorf("duration too long (maximum 1 year)")
	}

	return time.Now().Add(totalDuration), nil
}
