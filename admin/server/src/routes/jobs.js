import express from 'express';
import {execRnx} from '../utils/rnxExecutor.js';

const router = express.Router();

// List all jobs
router.get('/', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['list', '--json'], {node});

        // Parse rnx list output
        let jobs = [];
        if (output && output.trim()) {
            try {
                const rawJobs = JSON.parse(output);
                if (!Array.isArray(rawJobs)) {
                    jobs = [];
                } else {
                    // Transform field names to match frontend interface
                    jobs = rawJobs.map(job => ({
                        ...job,
                        startTime: job.start_time,
                        endTime: job.end_time,
                        exitCode: job.exit_code
                    }));
                }
            } catch (e) {
                console.warn('Failed to parse JSON from rnx list:', e.message);
                jobs = [];
            }
        }

        res.json(jobs);
    } catch (error) {
        console.error('Failed to list jobs:', error);
        res.json([]);
    }
});

// Execute a new job
router.post('/execute', async (req, res) => {
    try {
        const {
            command,
            schedule,
            runtime,
            volumes,
            uploads,
            uploadDirs,
            network,
            maxCPU,
            maxMemory,
            maxIOBPS,
            cpuCores,
            envVars,
            secretEnvVars
        } = req.body;
        const node = req.query.node;

        if (!command) {
            return res.status(400).json({error: 'Command is required'});
        }

        const args = ['run'];

        // Add schedule if provided (using = format)
        if (schedule && schedule.trim()) {
            args.push(`--schedule=${schedule.trim()}`);
        }

        // Add resource limits (using = format)
        if (maxCPU) args.push(`--max-cpu=${maxCPU}`);
        if (maxMemory) args.push(`--max-memory=${maxMemory}`);
        if (maxIOBPS) args.push(`--max-iobps=${maxIOBPS}`);
        if (cpuCores) args.push(`--cpu-cores=${cpuCores}`);

        // Add runtime (using = format)
        if (runtime) args.push(`--runtime=${runtime}`);

        // Add network (using = format)
        if (network) args.push(`--network=${network}`);

        // Add volumes (using = format)
        if (volumes && volumes.length > 0) {
            volumes.forEach(volume => {
                args.push(`--volume=${volume}`);
            });
        }

        // Add file uploads (using = format)
        if (uploads && uploads.length > 0) {
            uploads.forEach(upload => {
                args.push(`--upload=${upload}`);
            });
        }

        // Add directory uploads (using = format)
        if (uploadDirs && uploadDirs.length > 0) {
            uploadDirs.forEach(uploadDir => {
                args.push(`--upload-dir=${uploadDir}`);
            });
        }

        // Add environment variables (using = format)
        if (envVars) {
            Object.entries(envVars).forEach(([key, value]) => {
                args.push(`--env=${key}=${value}`);
            });
        }

        // Add secret environment variables (using = format)
        if (secretEnvVars) {
            Object.entries(secretEnvVars).forEach(([key, value]) => {
                args.push(`--secret-env=${key}=${value}`);
            });
        }

        // Add the command and any arguments
        const commandParts = command.trim().split(/\s+/);
        args.push(...commandParts);

        const output = await execRnx(args, {node});

        // Try to parse the output to extract job ID
        let jobId = null;
        const lines = output.split('\n');
        for (const line of lines) {
            if (line.includes('ID:')) {
                jobId = line.split('ID:')[1]?.trim();
                break;
            }
        }

        res.json({
            success: true,
            output: output,
            jobId: jobId
        });
    } catch (error) {
        console.error('Failed to execute job:', error);
        res.status(500).json({error: error.message});
    }
});

// Get comprehensive job status using rnx status
router.get('/:jobId/status', async (req, res) => {
    try {
        const {jobId} = req.params;
        const node = req.query.node;
        const output = await execRnx(['status', jobId, '--json'], {node});
        const statusData = JSON.parse(output);
        res.json(statusData);
    } catch (error) {
        console.error(`Failed to get job status for ${req.params.jobId}:`, error);
        res.status(500).json({
            error: 'Failed to get job status',
            message: error.message,
            id: req.params.jobId
        });
    }
});

// Get job details
router.get('/:jobId', async (req, res) => {
    try {
        const {jobId} = req.params;
        const node = req.query.node;
        const output = await execRnx(['status', jobId, '--json'], {node});

        let jobDetails;
        if (output && output.trim()) {
            try {
                const rawJob = JSON.parse(output);

                // Map RNX response to expected Job interface
                jobDetails = {
                    id: rawJob.uuid || rawJob.jobUuid || rawJob.id || jobId,
                    command: rawJob.command || '',
                    args: rawJob.args || [],
                    status: rawJob.status || 'UNKNOWN',
                    startTime: rawJob.startTime || '',
                    endTime: rawJob.endTime || '',
                    scheduledTime: rawJob.scheduledTime || rawJob.scheduled_time || '',
                    duration: rawJob.duration || 0,
                    exitCode: rawJob.exitCode || rawJob.exit_code,
                    maxCPU: rawJob.maxCPU || rawJob.max_cpu || 0,
                    maxMemory: rawJob.maxMemory || rawJob.max_memory || 0,
                    maxIOBPS: rawJob.maxIOBPS || rawJob.max_iobps || 0,
                    cpuCores: rawJob.cpuCores || rawJob.cpu_cores || '',
                    runtime: rawJob.runtime || '',
                    network: rawJob.network || 'bridge',
                    volumes: rawJob.volumes || [],
                    uploads: rawJob.uploads || [],
                    uploadDirs: rawJob.uploadDirs || rawJob.upload_dirs || [],
                    envVars: rawJob.environment || {},
                    secretEnvVars: {},
                    dependsOn: rawJob.dependsOn || rawJob.depends_on || [],
                    resourceUsage: rawJob.resourceUsage || rawJob.resource_usage
                };
            } catch (e) {
                console.warn('Failed to parse JSON from rnx status:', e.message);
                jobDetails = {
                    id: jobId,
                    status: 'UNKNOWN',
                    message: output || 'No output from status command'
                };
            }
        } else {
            jobDetails = {
                id: jobId,
                status: 'NOT_FOUND',
                message: 'Job not found or no output'
            };
        }

        res.json(jobDetails);
    } catch (error) {
        console.error(`Failed to get job ${req.params.jobId}:`, error);
        res.status(500).json({
            error: 'Failed to get job details',
            message: error.message,
            id: req.params.jobId,
            status: 'ERROR'
        });
    }
});

// Stop a job
router.post('/:jobId/stop', async (req, res) => {
    try {
        const {jobId} = req.params;
        const node = req.query.node;
        const output = await execRnx(['stop', jobId], {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to stop job:', error);
        res.status(500).json({error: error.message});
    }
});

// Delete a job
router.delete('/:jobId', async (req, res) => {
    try {
        const {jobId} = req.params;
        const node = req.query.node || 'default';

        const output = await execRnx(['delete', jobId], {node});

        res.json({
            message: `Job ${jobId} deleted successfully`,
            output: output.trim()
        });
    } catch (error) {
        console.error('Failed to delete job:', error);
        res.status(500).json({
            error: 'Failed to delete job',
            message: error.message
        });
    }
});

// Delete all non-running jobs
router.delete('/', async (req, res) => {
    try {
        const node = req.query.node || 'default';
        const output = await execRnx(['delete-all'], {node});

        res.json({
            message: 'All non-running jobs deleted successfully',
            output: output.trim()
        });
    } catch (error) {
        console.error('Failed to delete all jobs:', error);
        res.status(500).json({
            error: 'Failed to delete all jobs',
            message: error.message
        });
    }
});

export default router;