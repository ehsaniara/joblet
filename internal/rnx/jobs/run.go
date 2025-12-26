package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ehsaniara/joblet/internal/rnx/common"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	pkgconfig "github.com/ehsaniara/joblet/pkg/config"

	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a new job immediately or schedule it for later",
		Long: `Run a new job with the specified command and arguments, either immediately or scheduled for future execution.

Examples:
  # Immediate execution with Default network (bridge)
  rnx job run nginx
  rnx job run python3 script.py
  rnx job run bash -c "curl https://example.com"
  rnx --node=srv1 job run ps aux
  
  # No network
  rnx job run --network=none python3 process_local.py
  
  # Isolated network (external only)
  rnx job run --network=isolated wget https://example.com
  
  # Custom network with automatic hostname (job_<jobid>)
  rnx job run --network=backend python3 api.py
  rnx job run --network=backend postgres
  
  # With other flags
  rnx job run --network=frontend --max-cpu=50 --max-memory=512 node app.js

  # Scheduled execution
  rnx job run --schedule="1hour" python3 script.py
  rnx job run --schedule="30min" echo "Hello World"
  rnx job run --schedule="2025-07-18T20:02:48" backup_script.sh
  rnx job run --schedule="2h30m" --max-memory=512 data_processing.py

File Upload Examples:
  # Uploads work with both immediate and scheduled jobs (auto-detects files vs directories)
  rnx job run --upload=script.py python3 script.py
  rnx job run --upload=./myproject python3 main.py
  rnx job run --upload=data.csv --upload=./src python3 process.py

Volume Examples:
  # Use persistent volumes to share data between jobs
  rnx job run --volume=backend --upload=App1.jar java -jar App1.jar
  rnx job run --volume=backend --upload=App2.jar java -jar App2.jar
  rnx job run --volume=cache --volume=data python3 process.py

Runtime Examples:
  # Use pre-built runtime environments for fast job startup
  rnx job run --runtime=python-3.11-ml --upload=script.py python script.py
  rnx job run --runtime=openjdk-21 java -jar myapp.jar
  rnx job run --runtime=python-3.11-ml python train_model.py
  rnx job run --runtime=graalvmjdk-21 --upload=App.java java App.java

Environment Variable Examples:
  # Pass regular environment variables (visible in logs)
  rnx job run --env=NODE_ENV=production --env=PORT=8080 node app.js
  rnx job run -e DEBUG=true python app.py

  # Pass secret environment variables (hidden from logs)
  rnx job run --secret-env=API_KEY=secret123 --secret-env=DB_PASSWORD=pass123 python app.py
  rnx job run -s JWT_SECRET=mysecret node server.js

  # Combine different types
  rnx job run --env=NODE_ENV=prod --secret-env=API_KEY=secret node app.js

GPU Examples:
  # Request a single GPU for machine learning workloads
  rnx job run --gpu=1 python train_model.py
  rnx job run --gpu=1 --runtime=python-3.11-ml python pytorch_training.py

  # Request multiple GPUs with memory requirements
  rnx job run --gpu=2 --gpu-memory=16GB python distributed_training.py
  rnx job run --gpu=4 --gpu-memory=8GB --upload=model.py python model.py

  # GPU with other resource limits
  rnx job run --gpu=1 --max-memory=4096 --max-cpu=200 python inference.py

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
  --upload=PATH       Upload file or directory to job workspace (auto-detects)
  --runtime=SPEC      Use pre-built runtime (e.g., openjdk-21, python-3.11-ml)
  --volume=NAME       Mount persistent volume
  --network=NAME      Use network configuration
  --env=KEY=VALUE         Set environment variable (visible in logs)
  -e KEY=VALUE            Short form of --env
  --secret-env=KEY=VALUE  Set secret environment variable (hidden from logs)
  -s KEY=VALUE            Short form of --secret-env
  --gpu=N             Request N GPUs for the job (requires GPU support enabled)
  --gpu-memory=SIZE   Minimum GPU memory required (e.g., 8GB, 1024MB, 2048)`,
		Args:               cobra.MinimumNArgs(1),
		RunE:               runRun,
		DisableFlagParsing: true,
	}

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	// Handle help requests manually since DisableFlagParsing is true
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return cmd.Help()
		}
	}
	var (
		maxCPU        int32
		cpuCores      string
		maxMemory     int32
		maxIOBPS      int32
		uploads       []string
		schedule      string
		network       string
		volumes       []string
		runtime       string
		envVars       []string
		secretEnvVars []string
		gpuCount      int32
		gpuMemoryMB   int32
	)

	commandStartIndex := -1

	// Process arguments manually since DisableFlagParsing is enabled
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--config=") {
			common.ConfigPath = strings.TrimPrefix(arg, "--config=")
		} else if arg == "--config" && i+1 < len(args) {
			common.ConfigPath = args[i+1]
			i++ // Skip the next argument since we consumed it
		} else if strings.HasPrefix(arg, "--node=") {
			common.NodeName = strings.TrimPrefix(arg, "--node=")
		} else if arg == "--node" && i+1 < len(args) {
			common.NodeName = args[i+1]
			i++ // Skip the next argument since we consumed it
		} else if arg == "--json" {
			common.JSONOutput = true
		} else if strings.HasPrefix(arg, "--schedule=") {
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
		} else if strings.HasPrefix(arg, "--network=") {
			network = strings.TrimPrefix(arg, "--network=")
		} else if strings.HasPrefix(arg, "--volume=") {
			volumeName := strings.TrimPrefix(arg, "--volume=")
			volumes = append(volumes, volumeName)
		} else if strings.HasPrefix(arg, "--runtime=") {
			runtime = strings.TrimPrefix(arg, "--runtime=")
		} else if strings.HasPrefix(arg, "--env=") || strings.HasPrefix(arg, "-e=") {
			envVar := strings.TrimPrefix(arg, "--env=")
			if strings.HasPrefix(arg, "-e=") {
				envVar = strings.TrimPrefix(arg, "-e=")
			}
			envVars = append(envVars, envVar)
		} else if arg == "--env" || arg == "-e" {
			if i+1 < len(args) {
				envVars = append(envVars, args[i+1])
				i++ // Skip the next argument
			}
		} else if strings.HasPrefix(arg, "--secret-env=") || strings.HasPrefix(arg, "-s=") {
			secretEnvVar := strings.TrimPrefix(arg, "--secret-env=")
			if strings.HasPrefix(arg, "-s=") {
				secretEnvVar = strings.TrimPrefix(arg, "-s=")
			}
			secretEnvVars = append(secretEnvVars, secretEnvVar)
		} else if arg == "--secret-env" || arg == "-s" {
			if i+1 < len(args) {
				secretEnvVars = append(secretEnvVars, args[i+1])
				i++ // Skip the next argument
			}
		} else if strings.HasPrefix(arg, "--gpu=") {
			if val, err := parseIntFlag(arg, "--gpu="); err == nil {
				gpuCount = int32(val)
			}
		} else if strings.HasPrefix(arg, "--gpu-memory=") {
			gpuMemoryStr := strings.TrimPrefix(arg, "--gpu-memory=")
			// Parse GPU memory - support formats like "8GB", "1024MB", or just "1024"
			if val, err := parseGPUMemory(gpuMemoryStr); err == nil {
				gpuMemoryMB = int32(val)
			}
		} else if arg == "--" {
			// -- separator found, command starts at next position
			if i+1 < len(args) {
				commandStartIndex = i + 1
			}
			break
		} else if !strings.HasPrefix(arg, "--") {
			commandStartIndex = i
			break
		} else {
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if commandStartIndex < 0 || commandStartIndex >= len(args) {
		return fmt.Errorf("must specify a command to run")
	}

	commandArgs := args[commandStartIndex:]
	command := commandArgs[0]
	cmdArgs := commandArgs[1:]

	// Load client configuration manually since PersistentPreRun doesn't run with DisableFlagParsing
	var err error
	common.NodeConfig, err = pkgconfig.LoadClientConfig(common.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load client config: %w", err)
	}

	// Client creation using unified config
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Process file uploads
	fileUploads, err := processFileUploads(uploads)
	if err != nil {
		return fmt.Errorf("file upload processing failed: %w", err)
	}

	// Process environment variables
	environment, err := processEnvironmentVariables(envVars)
	if err != nil {
		return fmt.Errorf("environment variable processing failed: %w", err)
	}

	// Process secret environment variables
	secretEnvironment, err := processEnvironmentVariables(secretEnvVars)
	if err != nil {
		return fmt.Errorf("secret environment variable processing failed: %w", err)
	}

	// Display upload summary if files are being uploaded
	if len(fileUploads) > 0 {
		totalSize := int64(0)
		for _, upload := range fileUploads {
			totalSize += int64(len(upload.Content))
		}

		fmt.Printf("Uploading %d files (%.2f MB)...\n",
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
	request := &pb.RunJobRequest{
		Command:           command,
		Args:              cmdArgs,
		MaxCpu:            maxCPU,
		CpuCores:          cpuCores,
		MaxMemory:         maxMemory,
		MaxIobps:          maxIOBPS,
		Uploads:           fileUploads,
		Schedule:          scheduledTimeRFC3339,
		Network:           network,
		Volumes:           volumes,
		Runtime:           runtime,
		Environment:       environment,
		SecretEnvironment: secretEnvironment,
		GpuCount:          gpuCount,
		GpuMemoryMb:       gpuMemoryMB,
	}

	// Submit job
	response, err := jobClient.RunJob(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to run job: %v", err)
	}

	// Output JSON if requested
	if common.JSONOutput {
		return outputRunJobJSON(response, schedule, len(fileUploads), len(environment), len(secretEnvironment))
	}

	fmt.Printf("Job is running:\n")
	fmt.Printf("ID: %s\n", response.JobUuid)
	fmt.Printf("Command: %s %s\n", response.Command, strings.Join(response.Args, " "))
	// Display status with color coding
	statusColor, resetColor := getStatusColor(response.Status)
	fmt.Printf("Status: %s%s%s\n", statusColor, response.Status, resetColor)
	if schedule != "" {
		fmt.Printf("Schedule Input: %s\n", schedule) // Show user's original input
		fmt.Printf("Scheduled Time: %s\n", response.ScheduledTime)
	} else {
		fmt.Printf("StartTime: %s\n", response.StartTime)
	}

	if len(fileUploads) > 0 {
		fmt.Printf("Files: %d uploaded successfully\n", len(fileUploads))
	}

	if len(environment) > 0 {
		fmt.Printf("Environment: %d variables set\n", len(environment))
	}

	if len(secretEnvironment) > 0 {
		fmt.Printf("Secret Environment: %d variables set\n", len(secretEnvironment))
	}

	return nil
}

func parseIntFlag(arg, prefix string) (int, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.Atoi(valueStr)
}

func processFileUploads(uploads []string) ([]*pb.FileUpload, error) {
	var result []*pb.FileUpload

	for _, uploadPath := range uploads {
		fileInfo, err := os.Stat(uploadPath)
		if err != nil {
			return nil, fmt.Errorf("cannot access upload path %s: %w", uploadPath, err)
		}

		if fileInfo.IsDir() {
			// Auto-handle directories
			dirUploads, err := processDirectoryUpload(uploadPath)
			if err != nil {
				return nil, fmt.Errorf("directory upload failed for %s: %w", uploadPath, err)
			}
			result = append(result, dirUploads...)
		} else {
			// Handle single file
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

// processEnvironmentVariables processes environment variables from command line
func processEnvironmentVariables(envVars []string) (map[string]string, error) {
	environment := make(map[string]string)

	// Process individual environment variables (KEY=VALUE format)
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", envVar)
		}
		key := strings.TrimSpace(parts[0])
		value := parts[1] // Don't trim value as spaces might be intentional
		if key == "" {
			return nil, fmt.Errorf("environment variable key cannot be empty")
		}

		// Validate environment variable name
		if err := validateEnvironmentVariableName(key); err != nil {
			return nil, fmt.Errorf("invalid environment variable name '%s': %w", key, err)
		}

		// Validate environment variable value
		if err := validateEnvironmentVariableValue(key, value); err != nil {
			return nil, fmt.Errorf("invalid environment variable value for '%s': %w", key, err)
		}

		environment[key] = value
	}

	return environment, nil
}

// validateEnvironmentVariableName validates an environment variable name
func validateEnvironmentVariableName(name string) error {
	// Environment variable name validation regex
	// Valid names: letters, digits, underscore, must start with letter or underscore
	validNameRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("must start with letter or underscore and contain only letters, numbers, and underscores")
	}

	// Check for reserved environment variable names (warn only)
	if isReservedEnvironmentVariable(name) {
		fmt.Printf("Warning: '%s' is a reserved environment variable name\n", name)
	}

	return nil
}

// validateEnvironmentVariableValue validates an environment variable value
func validateEnvironmentVariableValue(name, value string) error {
	// Validate environment variable value length
	if len(value) > 32768 { // 32KB limit
		return fmt.Errorf("value is too long (%d bytes, max 32768)", len(value))
	}

	// Check for potentially dangerous values (warn only)
	if containsDangerousPatterns(value) {
		fmt.Printf("Warning: environment variable '%s' contains potentially dangerous patterns\n", name)
	}

	return nil
}

// isReservedEnvironmentVariable checks if an environment variable name is reserved by the system
func isReservedEnvironmentVariable(name string) bool {
	// List of common reserved environment variables
	reserved := map[string]bool{
		// System variables
		"PATH":     true,
		"HOME":     true,
		"USER":     true,
		"SHELL":    true,
		"TERM":     true,
		"PWD":      true,
		"OLDPWD":   true,
		"HOSTNAME": true,
		"LANG":     true,
		"LC_ALL":   true,

		// Joblet-specific variables (that might be set by the system)
		"JOBLET_JOB_ID":      true,
		"JOBLET_RUNTIME":     true,
		"JOBLET_VOLUME_PATH": true,
	}

	return reserved[name]
}

// containsDangerousPatterns performs basic security checks on environment variable values
func containsDangerousPatterns(value string) bool {
	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"$(",          // Command substitution
		"`",           // Backtick command substitution
		"rm -rf",      // Dangerous delete commands
		"format C:",   // Windows format command
		"del /f",      // Windows delete command
		"../",         // Path traversal attempt
		"passwd",      // Password file access
		"/etc/shadow", // Shadow file access
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerValue, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// outputRunJobJSON outputs the run job response in JSON format
func outputRunJobJSON(response *pb.RunJobResponse, scheduleInput string, fileCount, envCount, secretEnvCount int) error {
	// Create a structured response that includes additional context
	output := struct {
		JobUUID       string   `json:"job_uuid"`
		Command       string   `json:"command"`
		Args          []string `json:"args"`
		Status        string   `json:"status"`
		StartTime     string   `json:"start_time,omitempty"`
		ScheduleInput string   `json:"schedule_input,omitempty"`
		ScheduledTime string   `json:"scheduled_time,omitempty"`
		FilesUploaded int      `json:"files_uploaded,omitempty"`
		EnvVars       int      `json:"env_vars,omitempty"`
		SecretEnvVars int      `json:"secret_env_vars,omitempty"`
	}{
		JobUUID:       response.JobUuid,
		Command:       response.Command,
		Args:          response.Args,
		Status:        response.Status,
		StartTime:     response.StartTime,
		ScheduleInput: scheduleInput,
		ScheduledTime: response.ScheduledTime,
		FilesUploaded: fileCount,
		EnvVars:       envCount,
		SecretEnvVars: secretEnvCount,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// parseGPUMemory parses GPU memory specifications like "8GB", "1024MB", or "2048"
func parseGPUMemory(memoryStr string) (int, error) {
	memoryStr = strings.TrimSpace(strings.ToUpper(memoryStr))

	// Handle simple numeric values (assume MB)
	if val, err := strconv.Atoi(memoryStr); err == nil {
		return val, nil
	}

	// Handle values with units
	if strings.HasSuffix(memoryStr, "GB") {
		valueStr := strings.TrimSuffix(memoryStr, "GB")
		if val, err := strconv.Atoi(valueStr); err == nil {
			return val * 1024, nil // Convert GB to MB
		}
	} else if strings.HasSuffix(memoryStr, "MB") {
		valueStr := strings.TrimSuffix(memoryStr, "MB")
		if val, err := strconv.Atoi(valueStr); err == nil {
			return val, nil
		}
	}

	return 0, fmt.Errorf("invalid GPU memory format: %s (expected formats: 8GB, 1024MB, or 2048)", memoryStr)
}
