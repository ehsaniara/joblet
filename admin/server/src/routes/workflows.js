import express from 'express';
import {execRnx} from '../utils/rnxExecutor.js';

const router = express.Router();

// List all workflows
router.get('/', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['list', '--workflow', '--json'], {node});

        let workflows = [];
        if (output && output.trim()) {
            try {
                workflows = JSON.parse(output);
                if (!Array.isArray(workflows)) {
                    workflows = [];
                }
            } catch (e) {
                console.warn('Failed to parse JSON from rnx list --workflow:', e.message);
                workflows = [];
            }
        }

        res.json(workflows);
    } catch (error) {
        console.error('Failed to list workflows:', error);
        res.json([]);
    }
});

// Browse workflow directories (must come BEFORE /:workflowId route)
router.get('/browse', async (req, res) => {
    try {
        const {path: requestedPath} = req.query;
        const fs = await import('fs');
        const path = await import('path');

        // Default to current working directory if no path provided
        const browsePath = requestedPath || process.cwd();

        // Validate path exists and is accessible
        try {
            const stats = fs.default.statSync(browsePath);
            if (!stats.isDirectory()) {
                return res.status(400).json({error: 'Path is not a directory'});
            }
        } catch (err) {
            return res.status(404).json({error: 'Directory not found or not accessible'});
        }

        // Read directory contents
        const items = fs.default.readdirSync(browsePath, {withFileTypes: true});

        const directories = [];
        const yamlFiles = [];
        const otherFiles = [];

        items.forEach(item => {
            if (item.isDirectory() && !item.name.startsWith('.')) {
                directories.push({
                    name: item.name,
                    path: path.default.join(browsePath, item.name),
                    type: 'directory'
                });
            } else if (item.isFile()) {
                const fileInfo = {
                    name: item.name,
                    path: path.default.join(browsePath, item.name),
                    type: 'file'
                };

                if (item.name.endsWith('.yaml') || item.name.endsWith('.yml')) {
                    fileInfo.selectable = true;
                    yamlFiles.push(fileInfo);
                } else {
                    fileInfo.selectable = false;
                    otherFiles.push(fileInfo);
                }
            }
        });

        // Get parent directory path
        const parentPath = browsePath !== path.default.dirname(browsePath) ? path.default.dirname(browsePath) : null;

        res.json({
            currentPath: browsePath,
            parentPath,
            directories: directories.sort((a, b) => a.name.localeCompare(b.name)),
            yamlFiles: yamlFiles.sort((a, b) => a.name.localeCompare(b.name)),
            otherFiles: otherFiles.sort((a, b) => a.name.localeCompare(b.name))
        });
    } catch (error) {
        console.error('Failed to browse directory:', error);
        res.status(500).json({
            error: 'Failed to browse directory',
            message: error.message
        });
    }
});

// Validate workflow (must come BEFORE /:workflowId route)
router.post('/validate', async (req, res) => {
    try {
        const {filePath} = req.body;
        const node = req.query.node;

        if (!filePath) {
            return res.status(400).json({error: 'Workflow file path is required'});
        }

        // Validate file exists and is a YAML file
        const fs = await import('fs');
        const path = await import('path');

        try {
            const stats = fs.default.statSync(filePath);
            if (!stats.isFile()) {
                return res.status(400).json({error: 'Path is not a file'});
            }

            const ext = path.default.extname(filePath).toLowerCase();
            if (ext !== '.yaml' && ext !== '.yml') {
                return res.status(400).json({error: 'File must be a YAML file (.yaml or .yml)'});
            }
        } catch (err) {
            return res.status(404).json({error: 'Workflow file not found or not accessible'});
        }

        // Parse YAML to extract volume dependencies
        let missingVolumes = [];
        let allRequiredVolumes = [];

        try {
            // Read and parse the YAML file
            const yaml = await import('yaml');
            const fileContent = fs.default.readFileSync(filePath, 'utf8');
            const workflowData = yaml.default.parse(fileContent);

            // Extract volumes from all jobs
            const volumeSet = new Set();
            if (workflowData.jobs) {
                Object.values(workflowData.jobs).forEach(job => {
                    if (job.volumes && Array.isArray(job.volumes)) {
                        job.volumes.forEach(volume => volumeSet.add(volume));
                    }
                });
            }

            allRequiredVolumes = Array.from(volumeSet);

            // Check which volumes exist
            if (allRequiredVolumes.length > 0) {
                try {
                    const volumesOutput = await execRnx(['volume', 'list', '--json'], {node});
                    let existingVolumes = [];

                    if (volumesOutput && volumesOutput.trim()) {
                        const volumeData = JSON.parse(volumesOutput);
                        existingVolumes = volumeData.volumes || [];
                    }

                    const existingVolumeNames = existingVolumes.map(v => v.name);
                    missingVolumes = allRequiredVolumes.filter(vol => !existingVolumeNames.includes(vol));
                } catch (volumeError) {
                    // If volume listing fails, assume all volumes are missing
                    missingVolumes = allRequiredVolumes;
                }
            }

            res.json({
                valid: missingVolumes.length === 0,
                requiredVolumes: allRequiredVolumes,
                missingVolumes: missingVolumes,
                message: missingVolumes.length > 0
                    ? `Missing required volumes: ${missingVolumes.join(', ')}`
                    : 'All dependencies satisfied'
            });
        } catch (parseError) {
            res.status(400).json({
                error: 'Failed to parse workflow file',
                message: parseError.message
            });
        }
    } catch (error) {
        console.error('Failed to validate workflow:', error);
        res.status(500).json({
            error: 'Failed to validate workflow',
            message: error.message
        });
    }
});

// Execute workflow (must come BEFORE /:workflowId route)
router.post('/execute', async (req, res) => {
    try {
        const {filePath, workflowName, createMissingVolumes = false} = req.body;
        const node = req.query.node;

        if (!filePath) {
            return res.status(400).json({error: 'Workflow file path is required'});
        }

        // Validate file exists and is a YAML file
        const fs = await import('fs');
        const path = await import('path');

        try {
            const stats = fs.default.statSync(filePath);
            if (!stats.isFile()) {
                return res.status(400).json({error: 'Path is not a file'});
            }

            const ext = path.default.extname(filePath).toLowerCase();
            if (ext !== '.yaml' && ext !== '.yml') {
                return res.status(400).json({error: 'File must be a YAML file (.yaml or .yml)'});
            }
        } catch (err) {
            return res.status(404).json({error: 'Workflow file not found or not accessible'});
        }

        // If requested, create missing volumes first
        if (createMissingVolumes) {
            try {
                // Parse YAML to extract volume dependencies and create them
                const yaml = await import('yaml');
                const fileContent = fs.default.readFileSync(filePath, 'utf8');
                const workflowData = yaml.default.parse(fileContent);

                // Extract volumes from all jobs
                const volumeSet = new Set();
                if (workflowData.jobs) {
                    Object.values(workflowData.jobs).forEach(job => {
                        if (job.volumes && Array.isArray(job.volumes)) {
                            job.volumes.forEach(volume => volumeSet.add(volume));
                        }
                    });
                }

                // Create volumes that don't exist
                for (const volumeName of volumeSet) {
                    try {
                        await execRnx(['volume', 'create', volumeName], {node});
                    } catch (createError) {
                        console.warn(`Failed to create volume ${volumeName}:`, createError.message);
                    }
                }
            } catch (volumeError) {
                console.warn('Failed to create volumes:', volumeError.message);
            }
        }

        try {
            // Execute the workflow directly from the file path
            // If workflowName is provided, use the format file:workflowName
            const workflowArg = workflowName ? `${filePath}:${workflowName}` : filePath;
            const output = await execRnx(['run', '--workflow', workflowArg], {node});

            // Extract workflow UUID from output
            let workflowId = `workflow-${Date.now()}`;
            if (output) {
                // Try to match UUID format first (both full and short form)
                const uuidMatch = output.match(/(?:Workflow UUID|Workflow ID):\s*([a-f0-9-]{8,36})/i);
                if (uuidMatch && uuidMatch[1]) {
                    workflowId = uuidMatch[1];
                } else {
                    // Fallback to numeric ID format for backward compatibility
                    const idMatch = output.match(/Workflow ID:\s*(\d+)/);
                    if (idMatch && idMatch[1]) {
                        workflowId = idMatch[1];
                    }
                }
            }

            res.json({
                workflowId,
                status: 'created',
                message: 'Workflow created and started successfully',
                filePath,
                output: output
            });
        } catch (error) {
            // Check if this is a "multiple workflows found" error
            if (error.message && error.message.includes('multiple workflows found')) {
                // Extract workflow names from the error message
                const workflowMatch = error.message.match(/multiple workflows found: ([^.]+)/);
                let availableWorkflows = [];
                if (workflowMatch) {
                    availableWorkflows = workflowMatch[1].split(', ');
                }
                return res.status(400).json({
                    error: 'Multi-workflow files not supported',
                    message: `This YAML file contains multiple workflows (${availableWorkflows.join(', ')}). Please use separate YAML files for each workflow, or specify a single workflow using the format: ${filePath}:workflow-name`,
                    availableWorkflows,
                    multiWorkflowFile: true
                });
            }
            throw error;
        }
    } catch (error) {
        console.error('Failed to execute workflow:', error);
        res.status(500).json({
            error: 'Failed to execute workflow',
            message: error.message
        });
    }
});

// Get workflow details (must come AFTER specific routes like /browse, /validate, /execute)
router.get('/:workflowId', async (req, res) => {
    try {
        const {workflowId} = req.params;
        const node = req.query.node;

        // Get detailed workflow status including jobs
        let workflowData;
        try {
            const output = await execRnx(['status', '--workflow', workflowId, '--json'], {node});
            workflowData = JSON.parse(output);
        } catch (statusError) {
            // If status fails, try to get from list
            const workflowsOutput = await execRnx(['list', '--workflow', '--json'], {node});
            let workflows = [];
            if (workflowsOutput && workflowsOutput.trim()) {
                workflows = JSON.parse(workflowsOutput);
            }

            const workflow = workflows.find(w =>
                (w.uuid && w.uuid.toString() === workflowId) ||
                (w.workflowUuid && w.workflowUuid.toString() === workflowId) ||
                (w.id && w.id.toString() === workflowId)
            );
            if (!workflow) {
                return res.status(404).json({error: 'Workflow not found'});
            }

            workflowData = workflow;
        }

        // Transform workflow data to match the UI's WorkflowJob interface
        // First pass: create basic job objects with individual timing data
        const jobsWithBasicInfo = await Promise.all((workflowData.jobs || []).map(async (job, index) => {
            let jobTiming = {};

            // Fetch individual job details to get accurate timing information
            if (job.id) {
                try {
                    const jobOutput = await execRnx(['status', job.id, '--json'], {node});
                    const jobDetails = JSON.parse(jobOutput);
                    jobTiming = {
                        startTime: jobDetails.startTime || (jobDetails.start_time ? new Date(jobDetails.start_time).toISOString() : undefined),
                        endTime: jobDetails.endTime || (jobDetails.end_time ? new Date(jobDetails.end_time).toISOString() : undefined),
                        duration: jobDetails.duration || 0,
                        exitCode: jobDetails.exitCode || jobDetails.exit_code,
                        command: jobDetails.command || 'unknown',
                        args: jobDetails.args || []
                    };
                } catch (jobError) {
                    console.warn(`Failed to fetch timing for job ${job.id}:`, jobError.message);
                    // Fallback to workflow timing
                    jobTiming = {
                        startTime: workflowData.started_at ? new Date(workflowData.started_at.seconds * 1000).toISOString() : new Date().toISOString(),
                        endTime: workflowData.completed_at ? new Date(workflowData.completed_at.seconds * 1000).toISOString() : undefined,
                        duration: 0,
                        command: 'unknown',
                        args: []
                    };
                }
            }

            return {
                // Core Job interface fields
                id: job.jobUuid || job.id || `${job.name || 'job'}-${index}`,
                command: jobTiming.command || 'unknown',
                args: jobTiming.args || [],
                status: job.status || 'UNKNOWN',
                startTime: jobTiming.startTime,
                endTime: jobTiming.endTime,
                duration: jobTiming.duration || 0,
                exitCode: jobTiming.exitCode,
                maxCPU: 0,
                maxMemory: 0,
                maxIOBPS: 0,
                cpuCores: undefined,
                runtime: undefined,
                network: 'default',
                volumes: [],
                uploads: [],
                uploadDirs: [],
                envVars: {},
                secretEnvVars: {},
                dependsOn: [], // Will be populated in second pass

                // WorkflowJob extended fields
                name: job.name || `Job ${index + 1}`,
                rnxJobId: job.id || null,
                hasStarted: ['RUNNING', 'COMPLETED', 'FAILED', 'STOPPED'].includes(job.status),
                isWorkflowJob: true,
                workflowId: workflowData.uuid || workflowData.workflowUuid || workflowData.id,

                // Keep original dependencies for mapping
                originalDependencies: job.dependencies || []
            };
        }));

        // Create name-to-ID mapping
        const nameToIdMap = new Map();
        jobsWithBasicInfo.forEach(job => {
            nameToIdMap.set(job.name, job.id);
        });

        // Second pass: resolve dependency names to job IDs
        const transformedJobs = jobsWithBasicInfo.map(job => {
            const dependsOnIds = job.originalDependencies.map(depName => {
                const depId = nameToIdMap.get(depName);
                if (!depId) {
                    console.warn(`Dependency '${depName}' not found for job '${job.name}'`);
                    return depName; // Fallback to original name
                }
                return depId;
            });

            // Remove originalDependencies and set proper dependsOn and dependencies
            const {originalDependencies, ...finalJob} = job;
            return {
                ...finalJob,
                dependsOn: dependsOnIds,
                dependencies: job.originalDependencies // Keep original names for compatibility
            };
        });

        const transformedWorkflow = {
            ...workflowData,
            uuid: workflowData.uuid || workflowData.workflowUuid || workflowData.id,
            name: workflowData.workflow || `Workflow ${workflowData.uuid?.substring(0, 8) || workflowData.workflowUuid?.substring(0, 8) || 'Unknown'}`,
            jobs: transformedJobs
        };

        res.json(transformedWorkflow);
    } catch (error) {
        console.error('Failed to get workflow details:', error);
        res.status(500).json({error: error.message});
    }
});

// Get workflow YAML content
router.get('/:workflowId/yaml', async (req, res) => {
    try {
        const {workflowId} = req.params;
        const node = req.query.node;

        // Get workflow status with YAML content using --detail flag
        const output = await execRnx(['status', '--workflow', '--detail', workflowId, '--json'], {node});
        const workflowData = JSON.parse(output);

        if (!workflowData.yaml_content) {
            return res.status(404).json({
                error: 'YAML content not found',
                message: 'This workflow does not have YAML content available'
            });
        }

        res.json({
            content: workflowData.yaml_content,
            note: 'Original workflow YAML',
            lastModified: workflowData.created_at || new Date().toISOString(),
            workflowId: workflowData.uuid || workflowId,
            workflowName: workflowData.workflow
        });
    } catch (error) {
        console.error(`Failed to get workflow YAML ${req.params.workflowId}:`, error);
        res.status(500).json({
            error: 'Failed to get workflow YAML content',
            message: error.message
        });
    }
});

export default router;