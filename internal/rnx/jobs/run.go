package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"joblet/internal/rnx/common"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	pb "joblet/api/gen"
	"joblet/internal/joblet/workflow/types"
	"joblet/internal/rnx/workflows"
	pkgconfig "joblet/pkg/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a new job immediately or schedule it for later",
		Long: `Run a new job with the specified command and arguments, either immediately or scheduled for future execution.

Examples:
  # Immediate execution with Default network (bridge)
  rnx run nginx
  rnx run python3 script.py
  rnx run bash -c "curl https://example.com"
  rnx --node=srv1 run ps aux
  
  # No network
  rnx run --network=none python3 process_local.py
  
  # Isolated network (external only)
  rnx run --network=isolated wget https://example.com
  
  # Custom network with automatic hostname (job_<jobid>)
  rnx run --network=backend python3 api.py
  rnx run --network=backend postgres
  
  # With other flags
  rnx run --network=frontend --max-cpu=50 --max-memory=512 node app.js

  # Using YAML workflow for single job
  rnx run --workflow=jobs.yaml:analytics
  rnx run --workflow=deploy.yaml:production --args="v1.2.3"
  
  # Using YAML workflow for multi-job workflow (runs all jobs with dependencies)
  rnx run --workflow=ml-pipeline.yaml
  rnx run --workflow=workflow.yaml

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

Volume Examples:
  # Use persistent volumes to share data between jobs
  rnx run --volume=backend --upload=App1.jar java -jar App1.jar
  rnx run --volume=backend --upload=App2.jar java -jar App2.jar
  rnx run --volume=cache --volume=data python3 process.py

Runtime Examples:
  # Use pre-built runtime environments for fast job startup
  rnx run --runtime=python-3.11-ml --upload=script.py python script.py
  rnx run --runtime=openjdk-21 java -jar myapp.jar
  rnx run --runtime=python-3.11-ml python train_model.py
  rnx run --runtime=graalvmjdk-21 --upload=App.java java App.java

Environment Variable Examples:
  # Pass regular environment variables (visible in logs)
  rnx run --env=NODE_ENV=production --env=PORT=8080 node app.js
  rnx run -e DEBUG=true python app.py
  
  # Pass secret environment variables (hidden from logs)
  rnx run --secret-env=API_KEY=secret123 --secret-env=DB_PASSWORD=pass123 python app.py
  rnx run -s JWT_SECRET=mysecret node server.js
  
  # Combine different types
  rnx run --env=NODE_ENV=prod --secret-env=API_KEY=secret node app.js

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
  --workflow=FILE[:JOB] Load job or workflow configuration from YAML file
  --schedule=SPEC     Schedule job for future execution
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS
  --cpu-cores=SPEC    CPU cores specification
  --upload=FILE       Upload a file to the job workspace
  --upload-dir=DIR    Upload entire directory to the job workspace
  --runtime=SPEC      Use pre-built runtime (e.g., openjdk-21, python-3.11-ml)
  --volume=NAME       Mount persistent volume
  --network=NAME      Use network configuration
  --env=KEY=VALUE         Set environment variable (visible in logs)
  -e KEY=VALUE            Short form of --env
  --secret-env=KEY=VALUE  Set secret environment variable (hidden from logs)
  -s KEY=VALUE            Short form of --secret-env`,
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
		uploadDirs    []string
		schedule      string
		network       string
		volumes       []string
		runtime       string
		workflow      string
		envVars       []string
		secretEnvVars []string
	)

	commandStartIndex := -1

	// Process arguments manually since DisableFlagParsing is enabled
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--workflow=") {
			workflow = strings.TrimPrefix(arg, "--workflow=")
		} else if arg == "--workflow" && i+1 < len(args) {
			workflow = args[i+1]
			i++ // Skip the next argument since we consumed it
		} else if strings.HasPrefix(arg, "--config=") {
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
		} else if strings.HasPrefix(arg, "--upload-dir=") {
			uploadDir := strings.TrimPrefix(arg, "--upload-dir=")
			uploadDirs = append(uploadDirs, uploadDir)
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

	// Handle workflow loading if provided
	if workflow != "" {
		// Parse workflow spec to separate file and selector
		workflowParts := strings.SplitN(workflow, ":", 2)
		workflowFile := workflowParts[0]
		var workflowSelector string
		if len(workflowParts) > 1 {
			workflowSelector = workflowParts[1]
		}

		// First check if this is a workflow file and no job selector is provided
		if workflowSelector == "" {
			mode, _, err := tryWorkflowDetection(workflowFile)
			if err == nil && mode != workflows.ModeSingleJob {
				// This is a workflow - handle it differently
				var cmdArgs []string
				if commandStartIndex >= 0 && commandStartIndex < len(args) {
					cmdArgs = args[commandStartIndex:]
				}
				return handleWorkflowExecution(workflowFile, mode, "", cmdArgs)
			}
		} else {
			// If selector provided, check if it's a workflow selector in a multi-workflow file
			config, err := workflows.LoadWorkflowConfig(workflowFile)
			if err == nil && config.Workflows != nil && len(config.Workflows) > 0 {
				// Check if selector refers to a workflow name
				if _, exists := config.Workflows[workflowSelector]; exists {
					var cmdArgs []string
					if commandStartIndex >= 0 && commandStartIndex < len(args) {
						cmdArgs = args[commandStartIndex:]
					}
					return handleWorkflowExecution(workflowFile, workflows.ModeNamedWorkflow, workflowSelector, cmdArgs)
				}
				// If not a workflow name, let it fall through to regular job execution
			}
		}

		// Continue with regular single job workflow handling
		jobConfig, err := loadWorkflowConfig(workflow)
		if err != nil {
			return fmt.Errorf("failed to load workflow: %w", err)
		}

		// Apply workflow configuration
		if jobConfig.Resources.MaxCPU > 0 && maxCPU == 0 {
			maxCPU = int32(jobConfig.Resources.MaxCPU)
		}
		if jobConfig.Resources.MaxMemory > 0 && maxMemory == 0 {
			maxMemory = int32(jobConfig.Resources.MaxMemory)
		}
		if jobConfig.Resources.MaxIOBPS > 0 && maxIOBPS == 0 {
			maxIOBPS = int32(jobConfig.Resources.MaxIOBPS)
		}
		if jobConfig.Resources.CPUCores != "" && cpuCores == "" {
			cpuCores = jobConfig.Resources.CPUCores
		}
		if jobConfig.Network != "" && network == "" {
			network = jobConfig.Network
		}
		if jobConfig.Runtime != "" && runtime == "" {
			runtime = jobConfig.Runtime
		}
		if jobConfig.Schedule != "" && schedule == "" {
			schedule = jobConfig.Schedule
		}

		// Append volumes from workflow
		volumes = append(volumes, jobConfig.Volumes...)

		// Append uploads from workflow
		uploads = append(uploads, jobConfig.Uploads.Files...)
		uploadDirs = append(uploadDirs, jobConfig.Uploads.Directories...)

		// If no command provided in args, use workflow command
		if commandStartIndex < 0 && jobConfig.Command != "" {
			commandArgs := []string{jobConfig.Command}
			commandArgs = append(commandArgs, jobConfig.Args...)
			args = append(args, commandArgs...)
			commandStartIndex = len(args) - len(commandArgs)
		}
	}

	if commandStartIndex < 0 || commandStartIndex >= len(args) {
		return fmt.Errorf("must specify a command or use --workflow with a job definition")
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
	fileUploads, err := processFileUploads(uploads, uploadDirs)
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

	fmt.Printf("Job started:\n")
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

// loadWorkflowConfig loads a job configuration from a YAML workflow file
func loadWorkflowConfig(workflowSpec string) (*workflows.JobConfig, error) {
	// Parse workflow spec (format: file.yaml or file.yaml:jobname)
	parts := strings.SplitN(workflowSpec, ":", 2)
	workflowFile := parts[0]
	jobName := ""
	if len(parts) > 1 {
		jobName = parts[1]
	}

	// Load the YAML configuration
	jobSet, err := workflows.LoadConfig(workflowFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow file %s: %w", workflowFile, err)
	}

	// If no job name specified and there's only one job, use it
	if jobName == "" {
		if len(jobSet.Jobs) == 1 {
			for _, job := range jobSet.Jobs {
				return &job, nil
			}
		} else if len(jobSet.Jobs) > 1 {
			var jobNames []string
			for name := range jobSet.Jobs {
				jobNames = append(jobNames, name)
			}
			return nil, fmt.Errorf("multiple jobs found in workflow, please specify one: %s", strings.Join(jobNames, ", "))
		} else {
			// Return empty config with defaults applied
			return &jobSet.Defaults, nil
		}
	}

	// Look for the specified job
	if job, exists := jobSet.Jobs[jobName]; exists {
		return &job, nil
	}

	return nil, fmt.Errorf("job '%s' not found in workflow %s", jobName, workflowFile)
}

// tryWorkflowDetection attempts to detect if a workflow file contains workflows
func tryWorkflowDetection(workflowFile string) (workflows.WorkflowExecutionMode, string, error) {

	// Try to load as workflow file
	config, err := workflows.LoadWorkflowConfig(workflowFile)
	if err != nil {
		// If it fails to load as workflow, might be regular workflow
		return workflows.ModeSingleJob, "", err
	}

	// If we have multiple workflows, return appropriate mode
	if len(config.Workflows) > 0 {
		return workflows.ModeNamedWorkflow, "", nil
	}

	// Check for dependencies in jobs
	hasDependencies := false
	for _, job := range config.Jobs {
		if len(job.Requires) > 0 {
			hasDependencies = true
			break
		}
	}

	// If dependencies exist, it's a workflow
	if hasDependencies {
		return workflows.ModeWorkflow, "", nil
	}

	// Otherwise, parallel execution
	return workflows.ModeParallelJobs, "", nil
}

// handleWorkflowExecution handles workflow-based execution
func handleWorkflowExecution(workflowFile string, mode workflows.WorkflowExecutionMode, selector string, commandArgs []string) error {
	// Load client configuration for workflow execution
	var err error
	common.NodeConfig, err = pkgconfig.LoadClientConfig(common.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load client config for workflow: %w", err)
	}

	// Re-check the mode with the selector to determine actual execution path
	config, err := workflows.LoadWorkflowConfig(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to load workflow config: %w", err)
	}

	// If selector is provided, it should be a workflow name (job selectors are handled in regular path)
	if selector != "" {
		// Check if it's a workflow name
		if config.Workflows != nil {
			if _, exists := config.Workflows[selector]; exists {
				return executeWorkflow(workflowFile, selector, commandArgs)
			}
		}

		return fmt.Errorf("workflow '%s' not found in file", selector)
	}

	// No selector provided
	switch mode {
	case workflows.ModeWorkflow:
		return executeWorkflow(workflowFile, "", commandArgs)
	case workflows.ModeParallelJobs:
		return executeParallelJobs(workflowFile, commandArgs)
	default:
		// Check if we have multiple workflows and no selector
		if len(config.Workflows) > 0 {
			var workflowNames []string
			for name := range config.Workflows {
				workflowNames = append(workflowNames, name)
			}
			return fmt.Errorf("multiple workflows found: %s. Please specify which to run with workflow-file:workflow-name", strings.Join(workflowNames, ", "))
		}
		return fmt.Errorf("unsupported workflow mode")
	}
}

// executeWorkflow executes a workflow with dependencies
func executeWorkflow(workflowPath string, workflowName string, commandArgs []string) error {
	config, err := workflows.LoadWorkflowConfig(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to load workflow config: %w", err)
	}

	var jobs map[string]workflows.WorkflowJobConfig

	// Select the appropriate jobs based on mode
	if workflowName != "" {
		// Named workflow
		if workflow, exists := config.Workflows[workflowName]; exists {
			jobs = workflow.Jobs
		} else {
			return fmt.Errorf("workflow '%s' not found", workflowName)
		}
	} else {
		// Full workflow file as workflow
		jobs = config.Jobs
	}

	// Validate dependencies
	if err := workflows.ValidateDependencies(jobs); err != nil {
		return fmt.Errorf("invalid workflow dependencies: %w", err)
	}

	// Build dependency graph to ensure it's valid
	_, err = workflows.BuildDependencyGraph(jobs)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Execute the workflow using the workflow service
	return executeWorkflowViaService(workflowPath, workflowName)
}

// executeParallelJobs executes multiple jobs in parallel without dependencies
func executeParallelJobs(workflowPath string, commandArgs []string) error {
	config, err := workflows.LoadWorkflowConfig(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to load workflow config: %w", err)
	}

	if len(config.Jobs) == 0 {
		return fmt.Errorf("no jobs found in workflow")
	}

	// Execute parallel jobs as a workflow
	return executeWorkflowViaService(workflowPath, "")
}

// executeWorkflowViaService executes a workflow using the workflow service
func executeWorkflowViaService(workflowPath string, workflowName string) error {
	// Read and parse YAML file
	yamlContent, err := os.ReadFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file %s: %w", workflowPath, err)
	}

	var workflow types.WorkflowYAML
	if err := yaml.Unmarshal(yamlContent, &workflow); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate workflow before submission
	if err := validateWorkflowPreRequisites(workflow); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Extract and upload all files referenced in jobs
	workflowFiles, err := extractWorkflowFiles(workflowPath, workflow)
	if err != nil {
		return fmt.Errorf("failed to extract workflow files: %w", err)
	}

	fmt.Printf("Starting workflow from: %s\n", workflowPath)
	fmt.Printf("Found %d files to upload\n", len(workflowFiles))

	// Create client and workflow service
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	workflowClient := pb.NewJobServiceClient(client.GetConn())

	// Create workflow with YAML content and files
	createReq := &pb.RunWorkflowRequest{
		Workflow:      filepath.Base(workflowPath),
		YamlContent:   string(yamlContent),
		WorkflowFiles: workflowFiles,
		TotalJobs:     int32(len(workflow.Jobs)),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createRes, err := workflowClient.RunWorkflow(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	fmt.Printf("Workflow created with UUID: %s\n", createRes.WorkflowUuid)
	fmt.Printf("Use 'rnx status --workflow %s' to monitor progress\n", createRes.WorkflowUuid)

	return nil
}

// extractWorkflowFiles extracts and reads all files referenced in workflow jobs
func extractWorkflowFiles(yamlPath string, workflow types.WorkflowYAML) ([]*pb.FileUpload, error) {
	var uploads []*pb.FileUpload
	yamlDir := filepath.Dir(yamlPath)
	uploadedFiles := make(map[string]bool)

	// Collect all file uploads from all jobs
	for jobName, job := range workflow.Jobs {
		if job.Uploads != nil {
			for _, fileName := range job.Uploads.Files {
				if uploadedFiles[fileName] {
					continue // Skip duplicates
				}

				// Try relative to YAML file first, then absolute path
				filePath := filepath.Join(yamlDir, fileName)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					// Try absolute path
					filePath = fileName
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						return nil, fmt.Errorf("file %s referenced in job %s not found", fileName, jobName)
					}
				}

				// Read file content
				content, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
				}

				// Get file info
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
				}

				uploads = append(uploads, &pb.FileUpload{
					Path:        fileName,
					Content:     content,
					Mode:        uint32(fileInfo.Mode()),
					IsDirectory: false,
				})

				uploadedFiles[fileName] = true
			}
		}
	}

	return uploads, nil
}

// validateWorkflowPreRequisites performs comprehensive validation of workflow before submission
func validateWorkflowPreRequisites(workflow types.WorkflowYAML) error {
	// Fail-fast validation - stop immediately on first error

	// Check for circular dependencies
	if err := validateNonCircularDependencies(workflow); err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Validate all volumes exist
	if err := validateVolumesExist(workflow); err != nil {
		return fmt.Errorf("volume validation failed: %w", err)
	}

	// Validate all networks exist
	if err := validateNetworksExist(workflow); err != nil {
		return fmt.Errorf("network validation failed: %w", err)
	}

	// Validate all runtimes exist
	if err := validateRuntimesExist(workflow); err != nil {
		return fmt.Errorf("runtime validation failed: %w", err)
	}

	// Validate job dependencies reference existing jobs
	if err := validateJobDependencies(workflow); err != nil {
		return fmt.Errorf("job dependency validation failed: %w", err)
	}

	// Only show success output if ALL validations pass
	fmt.Println("Workflow validation completed successfully")
	return nil
}

// validateNonCircularDependencies checks for circular dependencies using DFS
func validateNonCircularDependencies(workflow types.WorkflowYAML) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for jobName, job := range workflow.Jobs {
		graph[jobName] = []string{}
		for _, req := range job.Requires {
			// req is map[string]string, iterate through key-value pairs
			for depJob := range req {
				if depJob != "expression" { // Skip expression entries for now
					graph[jobName] = append(graph[jobName], depJob)
				}
			}
		}
	}

	// Use DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var detectCycle func(string) error
	detectCycle = func(node string) error {
		visited[node] = true
		recStack[node] = true

		for _, dep := range graph[node] {
			if !visited[dep] {
				if err := detectCycle(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				return fmt.Errorf("circular dependency: %s -> %s", node, dep)
			}
		}

		recStack[node] = false
		return nil
	}

	for jobName := range workflow.Jobs {
		if !visited[jobName] {
			if err := detectCycle(jobName); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractJobNamesFromExpression parses dependency expressions to extract job names
func extractJobNamesFromExpression(expr string) []string {
	var jobNames []string
	// Simple parsing - look for patterns like "job-name=STATUS" or "job-name IN (...)"
	tokens := strings.Fields(expr)
	for _, token := range tokens {
		// Remove operators and status values
		if strings.Contains(token, "=") {
			parts := strings.Split(token, "=")
			if len(parts) > 0 {
				jobName := strings.TrimSpace(parts[0])
				if jobName != "" && !isStatusOrOperator(jobName) {
					jobNames = append(jobNames, jobName)
				}
			}
		}
	}
	return jobNames
}

// isStatusOrOperator checks if a token is a status value or operator
func isStatusOrOperator(token string) bool {
	statuses := []string{"COMPLETED", "FAILED", "CANCELED", "STOPPED", "RUNNING", "PENDING", "SCHEDULED"}
	operators := []string{"AND", "OR", "NOT", "IN", "NOT_IN", "&&", "||", "!"}

	for _, status := range statuses {
		if token == status {
			return true
		}
	}
	for _, op := range operators {
		if token == op {
			return true
		}
	}
	return false
}

// validateVolumesExist checks that all referenced volumes exist
func validateVolumesExist(workflow types.WorkflowYAML) error {
	requiredVolumes := make(map[string]bool)

	// Collect all volumes referenced in jobs
	for _, job := range workflow.Jobs {
		for _, volume := range job.Volumes {
			requiredVolumes[volume] = true
		}
	}

	if len(requiredVolumes) == 0 {
		return nil // No volumes required
	}

	// Get existing volumes from server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	volumeClient := pb.NewVolumeServiceClient(client.GetConn())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listRes, err := volumeClient.ListVolumes(ctx, &pb.EmptyRequest{})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	existingVolumes := make(map[string]bool)
	for _, volume := range listRes.Volumes {
		existingVolumes[volume.Name] = true
	}

	// Check if all required volumes exist
	var missingVolumes []string
	for volume := range requiredVolumes {
		if !existingVolumes[volume] {
			missingVolumes = append(missingVolumes, volume)
		}
	}

	if len(missingVolumes) > 0 {
		return fmt.Errorf("missing volumes: %v", missingVolumes)
	}

	return nil
}

// validateNetworksExist checks that all referenced networks exist
func validateNetworksExist(workflow types.WorkflowYAML) error {
	requiredNetworks := make(map[string]bool)

	// Collect all networks referenced in jobs
	for _, job := range workflow.Jobs {
		if job.Network != "" {
			requiredNetworks[job.Network] = true
		}
	}

	if len(requiredNetworks) == 0 {
		return nil // No custom networks required
	}

	// Get available networks from server
	availableNetworks := make(map[string]bool)

	// Built-in networks are always available
	builtinNetworks := []string{"none", "isolated", "bridge"}
	for _, network := range builtinNetworks {
		availableNetworks[network] = true
	}

	// Try to get custom networks from server
	jobClient, err := common.NewJobClient()
	if err != nil {
		// If we can't connect to server, just validate against built-in networks
		fmt.Printf("⚠️  Could not connect to server for network validation: %v\n", err)
	} else {
		defer jobClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := jobClient.ListNetworks(ctx)
		if err != nil {
			fmt.Printf("⚠️  Could not fetch custom networks: %v\n", err)
		} else {
			// Add custom networks to available list
			for _, network := range resp.Networks {
				availableNetworks[network.Name] = true
			}
		}
	}

	// Check each required network exists
	var missingNetworks []string
	for networkName := range requiredNetworks {
		if !availableNetworks[networkName] {
			missingNetworks = append(missingNetworks, networkName)
		}
	}

	if len(missingNetworks) > 0 {
		return fmt.Errorf("missing networks: %v. Available networks: %v",
			missingNetworks, getNetworkNames(availableNetworks))
	}

	return nil
}

// getNetworkNames extracts network names from a map for display
func getNetworkNames(networks map[string]bool) []string {
	var names []string
	for name := range networks {
		names = append(names, name)
	}
	return names
}

// validateRuntimesExist checks that all referenced runtimes exist
func validateRuntimesExist(workflow types.WorkflowYAML) error {
	requiredRuntimes := make(map[string]bool)

	// Collect all runtimes referenced in jobs
	for _, job := range workflow.Jobs {
		if job.Runtime != "" {
			requiredRuntimes[job.Runtime] = true
		}
	}

	if len(requiredRuntimes) == 0 {
		return nil // No custom runtimes required
	}

	// Get existing runtimes from server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	runtimeClient := pb.NewRuntimeServiceClient(client.GetConn())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listRes, err := runtimeClient.ListRuntimes(ctx, &pb.EmptyRequest{})
	if err != nil {
		return fmt.Errorf("failed to list runtimes: %w", err)
	}

	existingRuntimes := make(map[string]bool)
	for _, runtime := range listRes.Runtimes {
		// Store both original name and normalized name (colon format)
		existingRuntimes[runtime.Name] = true
		// Convert hyphen format to colon format (python-3.11-ml -> python-3.11-ml)
		normalizedName := strings.Replace(runtime.Name, "-", ":", 1)
		existingRuntimes[normalizedName] = true
	}

	// Check if all required runtimes exist
	var missingRuntimes []string
	for runtime := range requiredRuntimes {
		if !existingRuntimes[runtime] {
			missingRuntimes = append(missingRuntimes, runtime)
		}
	}

	if len(missingRuntimes) > 0 {
		return fmt.Errorf("missing runtimes: %v", missingRuntimes)
	}

	return nil
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
		"JOBLET_WORKFLOW_ID": true,
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

// validateJobDependencies checks that all job dependencies reference existing jobs
func validateJobDependencies(workflow types.WorkflowYAML) error {
	// Get all job names
	allJobs := make(map[string]bool)
	for jobName := range workflow.Jobs {
		allJobs[jobName] = true
	}

	// Check dependencies
	for jobName, job := range workflow.Jobs {
		for _, req := range job.Requires {
			// req is map[string]string, iterate through key-value pairs
			for depJob, status := range req {
				if depJob == "expression" {
					// Handle expression-based dependencies
					deps := extractJobNamesFromExpression(status)
					for _, dep := range deps {
						if !allJobs[dep] {
							return fmt.Errorf("job '%s' has expression dependency on non-existent job '%s'", jobName, dep)
						}
					}
				} else {
					// Handle direct job dependencies
					if !allJobs[depJob] {
						return fmt.Errorf("job '%s' depends on non-existent job '%s'", jobName, depJob)
					}
				}
			}
		}
	}

	return nil
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
