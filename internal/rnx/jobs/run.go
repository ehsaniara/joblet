package jobs

import (
	"context"
	"fmt"
	"joblet/internal/rnx/common"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	pb "joblet/api/gen"
	"joblet/internal/rnx/templates"
	pkgconfig "joblet/pkg/config"

	"github.com/spf13/cobra"
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

  # Using YAML template
  rnx run --template=jobs.yaml:analytics
  rnx run --template=ml-pipeline.yaml
  rnx run --template=deploy.yaml:production --args="v1.2.3"

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
  rnx run --runtime=python:3.11 --upload=script.py python script.py
  rnx run --runtime=java:17 --jar myapp.jar
  rnx run --runtime=python:3.11+ml+gpu python train_model.py
  rnx run --runtime=node:18 --upload=app.js node app.js

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
  --template=FILE[:JOB] Load job configuration from YAML file
  --schedule=SPEC     Schedule job for future execution
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS
  --cpu-cores=SPEC    CPU cores specification
  --upload=FILE       Upload a file to the job workspace
  --upload-dir=DIR    Upload entire directory to the job workspace
  --runtime=SPEC      Use pre-built runtime (e.g., python:3.11, java:17)
  --volume=NAME       Mount persistent volume
  --network=NAME      Use network configuration`,
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
		schedule   string
		network    string
		volumes    []string
		runtime    string
		template   string
	)

	commandStartIndex := -1

	// Process arguments manually since DisableFlagParsing is enabled
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if strings.HasPrefix(arg, "--template=") {
			template = strings.TrimPrefix(arg, "--template=")
		} else if arg == "--template" && i+1 < len(args) {
			template = args[i+1]
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

	// Handle template loading if provided
	if template != "" {
		// Parse template spec to separate file and selector
		templateParts := strings.SplitN(template, ":", 2)
		templateFile := templateParts[0]
		var templateSelector string
		if len(templateParts) > 1 {
			templateSelector = templateParts[1]
		}

		// First check if this is a workflow template and no job selector is provided
		if templateSelector == "" {
			mode, _, err := tryWorkflowDetection(templateFile)
			if err == nil && mode != templates.ModeSingleJob {
				// This is a workflow - handle it differently
				var cmdArgs []string
				if commandStartIndex >= 0 && commandStartIndex < len(args) {
					cmdArgs = args[commandStartIndex:]
				}
				return handleWorkflowExecution(templateFile, mode, "", cmdArgs)
			}
		} else {
			// If selector provided, check if it's a workflow selector in a multi-workflow template
			config, err := templates.LoadWorkflowConfig(templateFile)
			if err == nil && config.Workflows != nil && len(config.Workflows) > 0 {
				// Check if selector refers to a workflow name
				if _, exists := config.Workflows[templateSelector]; exists {
					var cmdArgs []string
					if commandStartIndex >= 0 && commandStartIndex < len(args) {
						cmdArgs = args[commandStartIndex:]
					}
					return handleWorkflowExecution(templateFile, templates.ModeNamedWorkflow, templateSelector, cmdArgs)
				}
				// If not a workflow name, let it fall through to regular job execution
			}
		}

		// Continue with regular single job template handling
		jobConfig, err := loadTemplateConfig(template)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Apply template configuration
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

		// Append volumes from template
		volumes = append(volumes, jobConfig.Volumes...)

		// Append uploads from template
		uploads = append(uploads, jobConfig.Uploads.Files...)
		uploadDirs = append(uploadDirs, jobConfig.Uploads.Directories...)

		// If no command provided in args, use template command
		if commandStartIndex < 0 && jobConfig.Command != "" {
			commandArgs := []string{jobConfig.Command}
			commandArgs = append(commandArgs, jobConfig.Args...)
			args = append(args, commandArgs...)
			commandStartIndex = len(args) - len(commandArgs)
		}
	}

	if commandStartIndex < 0 || commandStartIndex >= len(args) {
		return fmt.Errorf("must specify a command or use --template with a job definition")
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
	request := &pb.RunJobReq{
		Command:   command,
		Args:      cmdArgs,
		MaxCPU:    maxCPU,
		CpuCores:  cpuCores,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
		Uploads:   fileUploads,
		Schedule:  scheduledTimeRFC3339,
		Network:   network,
		Volumes:   volumes,
		Runtime:   runtime,
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

// loadTemplateConfig loads a job configuration from a YAML template file
func loadTemplateConfig(templateSpec string) (*templates.JobConfig, error) {
	// Parse template spec (format: file.yaml or file.yaml:jobname)
	parts := strings.SplitN(templateSpec, ":", 2)
	templateFile := parts[0]
	jobName := ""
	if len(parts) > 1 {
		jobName = parts[1]
	}

	// Load the YAML configuration
	jobSet, err := templates.LoadConfig(templateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load template file %s: %w", templateFile, err)
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
			return nil, fmt.Errorf("multiple jobs found in template, please specify one: %s", strings.Join(jobNames, ", "))
		} else {
			// Return empty config with defaults applied
			return &jobSet.Defaults, nil
		}
	}

	// Look for the specified job
	if job, exists := jobSet.Jobs[jobName]; exists {
		return &job, nil
	}

	return nil, fmt.Errorf("job '%s' not found in template %s", jobName, templateFile)
}

// tryWorkflowDetection attempts to detect if a template contains workflows
func tryWorkflowDetection(templateFile string) (templates.WorkflowExecutionMode, string, error) {

	// Try to load as workflow template
	config, err := templates.LoadWorkflowConfig(templateFile)
	if err != nil {
		// If it fails to load as workflow, might be regular template
		return templates.ModeSingleJob, "", err
	}

	// If we have multiple workflows, return appropriate mode
	if len(config.Workflows) > 0 {
		return templates.ModeNamedWorkflow, "", nil
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
		return templates.ModeWorkflow, "", nil
	}

	// Otherwise, parallel execution
	return templates.ModeParallelJobs, "", nil
}

// handleWorkflowExecution handles workflow-based template execution
func handleWorkflowExecution(templateFile string, mode templates.WorkflowExecutionMode, selector string, commandArgs []string) error {
	// Re-check the mode with the selector to determine actual execution path
	config, err := templates.LoadWorkflowConfig(templateFile)
	if err != nil {
		return fmt.Errorf("failed to load workflow config: %w", err)
	}

	// If selector is provided, it should be a workflow name (job selectors are handled in regular path)
	if selector != "" {
		// Check if it's a workflow name
		if config.Workflows != nil {
			if _, exists := config.Workflows[selector]; exists {
				return executeWorkflow(templateFile, selector, commandArgs)
			}
		}

		return fmt.Errorf("workflow '%s' not found in template", selector)
	}

	// No selector provided
	switch mode {
	case templates.ModeWorkflow:
		return executeWorkflow(templateFile, "", commandArgs)
	case templates.ModeParallelJobs:
		return executeParallelJobs(templateFile, commandArgs)
	default:
		// Check if we have multiple workflows and no selector
		if len(config.Workflows) > 0 {
			var workflowNames []string
			for name := range config.Workflows {
				workflowNames = append(workflowNames, name)
			}
			return fmt.Errorf("multiple workflows found: %s. Please specify which to run with template:workflow-name", strings.Join(workflowNames, ", "))
		}
		return fmt.Errorf("unsupported workflow mode")
	}
}

// executeWorkflow executes a workflow with dependencies
func executeWorkflow(templatePath string, workflowName string, commandArgs []string) error {
	config, err := templates.LoadWorkflowConfig(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load workflow config: %w", err)
	}

	var jobs map[string]templates.WorkflowJobConfig

	// Select the appropriate jobs based on mode
	if workflowName != "" {
		// Named workflow
		if workflow, exists := config.Workflows[workflowName]; exists {
			jobs = workflow.Jobs
		} else {
			return fmt.Errorf("workflow '%s' not found", workflowName)
		}
	} else {
		// Full template as workflow
		jobs = config.Jobs
	}

	// Validate dependencies
	if err := templates.ValidateDependencies(jobs); err != nil {
		return fmt.Errorf("invalid workflow dependencies: %w", err)
	}

	// Build dependency graph to ensure it's valid
	_, err = templates.BuildDependencyGraph(jobs)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Workflow execution via templates is deprecated
	// Direct users to use the workflow command instead
	return fmt.Errorf("workflow execution via --template is deprecated. Use 'rnx workflow run %s' instead", templatePath)
}

// executeParallelJobs executes multiple jobs in parallel without dependencies
func executeParallelJobs(templatePath string, commandArgs []string) error {
	config, err := templates.LoadWorkflowConfig(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load template config: %w", err)
	}

	if len(config.Jobs) == 0 {
		return fmt.Errorf("no jobs found in template")
	}

	// Parallel job execution via templates is deprecated
	// Direct users to use the workflow command instead
	return fmt.Errorf("parallel job execution via --template is deprecated. Use 'rnx workflow run %s' instead", templatePath)
}
