// Node.js version check - ensure we have Node.js 18+
const nodeVersion = process.version;
const majorVersion = parseInt(nodeVersion.slice(1).split('.')[0]);

if (majorVersion < 18) {
    console.error(`❌ Node.js ${nodeVersion} detected, but Node.js 18+ is required`);
    console.error(`💡 Please upgrade Node.js to version 18 or later`);
    console.error(`   Visit: https://nodejs.org/`);
    process.exit(1);
}

import express from 'express';
import {exec, spawn} from 'child_process';
import {promisify} from 'util';
import {WebSocketServer} from 'ws';
import {createServer} from 'http';
import path from 'path';
import {fileURLToPath} from 'url';
import cors from 'cors';

const execAsync = promisify(exec);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const server = createServer(app);
const wss = new WebSocketServer({server});

// Configuration
const PORT = process.env.PORT || 5173;
const BIND_ADDRESS = process.env.BIND_ADDRESS || 'localhost';
const RNX_PATH = process.env.RNX_PATH || '../../bin/rnx';

// Middleware
app.use(cors());
app.use(express.json());

// Serve static files from React build
app.use(express.static(path.join(__dirname, '../ui/dist')));

// Helper function to execute rnx commands
async function execRnx(args, options = {}) {
    try {
        // Add node selection if provided
        const node = options.node;
        if (node && node !== 'default') {
            args = ['--node', node, ...args];
        }

        const command = `${RNX_PATH} ${args.join(' ')}`;
        // console.log(`Executing: ${command}`);
        const {stdout, stderr} = await execAsync(command, options);
        if (stderr) {
            console.warn(`Command warning: ${stderr}`);
        }
        return stdout.trim();
    } catch (error) {
        console.error(`Command failed: ${error.message}`);
        throw error;
    }
}

// API Routes

// List available nodes
app.get('/api/nodes', async (req, res) => {
    try {
        const output = await execRnx(['nodes', '--json']);

        let nodes = [];
        if (output && output.trim()) {
            try {
                nodes = JSON.parse(output);
                if (!Array.isArray(nodes)) {
                    nodes = [];
                }
            } catch (e) {
                console.warn('Failed to parse JSON from rnx nodes:', e.message);
                nodes = [];
            }
        }

        // Always include default node if not present
        if (!nodes.find(n => n.name === 'default')) {
            nodes.unshift({name: 'default', status: 'active', default: true});
        }

        res.json(nodes);
    } catch (error) {
        console.error('Failed to list nodes:', error);
        // Return default node on error
        res.json([{name: 'default', status: 'active', default: true}]);
    }
});

// List all jobs
app.get('/api/jobs', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['list', '--json'], {node});

        // Parse rnx list output
        let jobs = [];
        if (output && output.trim()) {
            try {
                jobs = JSON.parse(output);
                // Ensure jobs is an array
                if (!Array.isArray(jobs)) {
                    jobs = [];
                }
            } catch (e) {
                console.warn('Failed to parse JSON from rnx list:', e.message);
                jobs = [];
            }
        }

        res.json(jobs);
    } catch (error) {
        console.error('Failed to list jobs:', error);
        // Return empty array instead of error to prevent UI crashes
        res.json([]);
    }
});

// Execute a new job
app.post('/api/jobs/execute', async (req, res) => {
    try {
        const {
            command,
            args = [],
            maxCPU,
            maxMemory,
            maxIOBPS,
            cpuCores,
            runtime,
            network,
            volumes = [],
            uploads = [],
            uploadDirs = [],
            envVars = {},
            secretEnvVars = {},
            schedule,
            node
        } = req.body;

        if (!command) {
            return res.status(400).json({error: 'Command is required'});
        }

        // Build rnx run command arguments - flags must come before the command
        const rnxArgs = ['run'];

        // Add optional flags first (before command)
        // NOTE: Some flags require = syntax instead of space-separated
        if (maxCPU) rnxArgs.push(`--max-cpu=${maxCPU.toString()}`);
        if (maxMemory) rnxArgs.push(`--max-memory=${maxMemory.toString()}`);
        if (maxIOBPS) rnxArgs.push(`--max-iobps=${maxIOBPS.toString()}`);
        if (cpuCores) rnxArgs.push(`--cpu-cores=${cpuCores}`);
        if (runtime) rnxArgs.push(`--runtime=${runtime}`);
        if (network) rnxArgs.push(`--network=${network}`);
        if (schedule) rnxArgs.push(`--schedule=${schedule}`);

        // Add volumes
        volumes.forEach(volume => {
            rnxArgs.push(`--volume=${volume}`);
        });

        // Add uploads
        uploads.forEach(upload => {
            rnxArgs.push(`--upload=${upload}`);
        });

        // Add upload directories
        uploadDirs.forEach(dir => {
            rnxArgs.push(`--upload-dir=${dir}`);
        });

        // Add environment variables
        Object.entries(envVars).forEach(([key, value]) => {
            rnxArgs.push(`--env=${key}=${value}`);
        });

        // Add secret environment variables
        Object.entries(secretEnvVars).forEach(([key, value]) => {
            rnxArgs.push(`--secret-env=${key}=${value}`);
        });

        // Add command and args last
        rnxArgs.push(command);
        if (args && args.length > 0) {
            rnxArgs.push(...args);
        }

        // Execute the job
        const output = await execRnx(rnxArgs, {node});

        // Extract job UUID from output (format may now include UUID instead of numeric ID)
        let jobId = `job-${Date.now()}`;
        if (output) {
            // Try to match UUID format first (both full and short form)
            const uuidMatch = output.match(/(?:Job UUID|ID):\s*([a-f0-9-]{8,36})/i);
            if (uuidMatch && uuidMatch[1]) {
                jobId = uuidMatch[1];
            } else {
                // Fallback to numeric ID format for backward compatibility
                const idMatch = output.match(/ID:\s*(\d+)/);
                if (idMatch && idMatch[1]) {
                    jobId = idMatch[1];
                }
            }
        }

        res.json({
            jobId,
            status: 'created',
            message: 'Job created successfully'
        });
    } catch (error) {
        console.error('Failed to execute job:', error);
        res.status(500).json({error: 'Failed to execute job', message: error.message});
    }
});

// Browse directories for workflow files (must come before /:workflowId route)
app.get('/api/workflows/browse', async (req, res) => {
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

// Get workflow details with jobs
app.get('/api/workflows/:workflowId', async (req, res) => {
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

        // Use real job UUIDs from RNX workflow status
        // RNX now provides actual job UUIDs in workflow status
        // NOTE: Non-starting and cancelled jobs may not have a UUID assigned

        // First pass: create a mapping of job names to UI IDs
        const jobNameToUiId = new Map();
        (workflowData.jobs || []).forEach((job, index) => {
            // Handle new UUID format: jobUuid instead of id
            const rnxJobUuid = job.jobUuid || job.id || null;
            let uiJobId;

            // For non-starting jobs (no UUID or null), create a unique UI identifier
            if (!rnxJobUuid) {
                const baseName = job.name || `job-${index}`;
                const jobStatus = job.status || 'PENDING';
                const statusSuffix = jobStatus.toLowerCase().replace(/[^a-z0-9]/g, '');
                uiJobId = `${baseName}-${statusSuffix}-${index}`;
            } else {
                // Use UUID directly as UI ID (UUIDs are already unique)
                uiJobId = rnxJobUuid;
            }

            jobNameToUiId.set(job.name || `job-${index}`, uiJobId);
        });

        // Second pass: create job objects with properly mapped dependencies
        const jobsWithDetails = (workflowData.jobs || []).map((job, index) => {
            // Handle new UUID format: jobUuid instead of id
            const rnxJobUuid = job.jobUuid || job.id || null;
            const jobStatus = job.status || 'UNKNOWN';

            // Create unique UI identifier
            let uiJobId;

            if (!rnxJobUuid) {
                // For jobs with no UUID (non-starting, cancelled, etc.)
                // Use job name + status + index to ensure uniqueness
                const baseName = job.name || `job-${index}`;
                const statusSuffix = jobStatus.toLowerCase().replace(/[^a-z0-9]/g, ''); // Clean status for ID
                uiJobId = `${baseName}-${statusSuffix}-${index}`;
            } else {
                // Use UUID directly as UI ID (UUIDs are already unique)
                uiJobId = rnxJobUuid;
            }

            // Determine if job has actually started executing
            // With UUIDs, jobs that have started will have a valid UUID
            const hasActuallyStarted = rnxJobUuid !== null &&
                ['RUNNING', 'COMPLETED', 'FAILED'].includes(jobStatus);

            // Map dependencies from job names to UI IDs
            const mappedDependencies = (job.dependencies || []).map(depName => {
                // Dependencies might be job names, so map them to UI IDs
                return jobNameToUiId.get(depName) || depName;
            });

            return {
                id: uiJobId, // UI identifier (guaranteed unique)
                rnxJobId: rnxJobUuid, // Original RNX job UUID (can be null for non-started jobs)
                name: job.name || `job-${index}`,
                status: jobStatus,
                dependsOn: mappedDependencies, // Dependencies mapped to UI IDs
                // Mark these as workflow jobs so we know they can be fetched individually
                isWorkflowJob: true,
                workflowId: workflowId,
                // Indicate if job has started executing (not just queued/pending/cancelled)
                hasStarted: hasActuallyStarted,
                // Use job name as command for display purposes in the graph
                command: job.name || uiJobId, // Show job name in the graph
                args: [],
                startTime: null, // Will be fetched from individual job status
                endTime: null,
                duration: 0,
                maxCPU: 0,
                maxMemory: 0,
                maxIOBPS: 0,
                network: 'bridge',
                volumes: [],
                envVars: {},
                secretEnvVars: {},
                exitCode: jobStatus === 'COMPLETED' ? 0 :
                    jobStatus === 'FAILED' ? 1 : undefined
            };
        });

        // Format workflow data for the UI
        const workflowWithJobs = {
            id: workflowData.workflowUuid || workflowData.id || workflowId,
            name: workflowData.workflow || workflowData.name || `Workflow ${workflowId}`,
            workflow: workflowData.workflow || workflowData.name || 'unknown',
            status: workflowData.status || 'UNKNOWN',
            total_jobs: workflowData.total_jobs || workflowData.jobs?.length || 0,
            completed_jobs: workflowData.completed_jobs || 0,
            failed_jobs: workflowData.failed_jobs || 0,
            created_at: workflowData.created_at?.seconds ?
                new Date(workflowData.created_at.seconds * 1000).toISOString() :
                workflowData.created_at || null,
            started_at: workflowData.started_at?.seconds ?
                new Date(workflowData.started_at.seconds * 1000).toISOString() :
                workflowData.start_time || null,
            completed_at: workflowData.completed_at?.seconds ?
                new Date(workflowData.completed_at.seconds * 1000).toISOString() :
                workflowData.end_time || null,
            jobs: jobsWithDetails
        };

        res.json(workflowWithJobs);
    } catch (error) {
        console.error(`Failed to get workflow ${req.params.workflowId}:`, error);
        res.status(500).json({
            error: 'Failed to get workflow details',
            message: error.message
        });
    }
});

// Get job details
app.get('/api/jobs/:jobId', async (req, res) => {
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
                    id: rawJob.jobUuid || rawJob.id || jobId,
                    command: rawJob.command || '',
                    args: rawJob.args || [],
                    status: rawJob.status || 'UNKNOWN',
                    startTime: rawJob.startTime || '',
                    endTime: rawJob.endTime || '',
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
                    secretEnvVars: {}, // Note: RNX doesn't distinguish between regular and secret vars in response
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

// Get comprehensive system information
app.get('/api/system-info', async (req, res) => {
    try {
        const node = req.query.node;

        // Get comprehensive system information using monitor status command
        const output = await execRnx(['monitor', 'status', '--json'], {
            node,
            maxBuffer: 1024 * 1024 * 10 // 10MB buffer
        });

        const monitorData = JSON.parse(output);

        // Transform the data to match the frontend DetailedSystemInfo interface
        const systemInfo = {
            hostInfo: {
                hostname: monitorData.host?.hostname,
                platform: monitorData.host?.os,
                arch: monitorData.host?.architecture,
                release: monitorData.host?.kernelVersion,
                uptime: monitorData.host?.uptime,
                cloudProvider: monitorData.cloud?.provider,
                instanceType: monitorData.cloud?.instanceType,
                region: monitorData.cloud?.region
            },
            cpuInfo: {
                cores: monitorData.cpu?.cores,
                model: 'N/A', // Not provided by monitor status
                frequency: null, // Not provided by monitor status
                usage: monitorData.cpu?.usagePercent,
                loadAverage: monitorData.cpu?.loadAverage,
                perCoreUsage: monitorData.cpu?.perCoreUsage,
                temperature: null // Not provided by monitor status
            },
            memoryInfo: {
                total: monitorData.memory?.totalBytes,
                used: monitorData.memory?.usedBytes,
                available: monitorData.memory?.availableBytes,
                percent: monitorData.memory?.usagePercent,
                buffers: monitorData.memory?.bufferedBytes,
                cached: monitorData.memory?.cachedBytes,
                swap: {
                    total: monitorData.memory?.swapTotal || 0,
                    used: (monitorData.memory?.swapTotal || 0) - (monitorData.memory?.swapFree || 0),
                    percent: monitorData.memory?.swapTotal > 0
                        ? ((monitorData.memory.swapTotal - monitorData.memory.swapFree) / monitorData.memory.swapTotal) * 100
                        : 0
                }
            },
            disksInfo: {
                disks: monitorData.disks?.map(disk => ({
                    name: disk.device,
                    mountpoint: disk.mountPoint,
                    filesystem: disk.filesystem,
                    size: disk.totalBytes,
                    used: disk.usedBytes,
                    available: disk.freeBytes,
                    percent: disk.usagePercent,
                    readBps: null, // Not provided by monitor status
                    writeBps: null, // Not provided by monitor status
                    iops: null // Not provided by monitor status
                })) || [],
                totalSpace: monitorData.disks?.reduce((total, disk) => total + (disk.totalBytes || 0), 0),
                usedSpace: monitorData.disks?.reduce((total, disk) => total + (disk.usedBytes || 0), 0)
            },
            networkInfo: {
                interfaces: monitorData.networks?.map(net => ({
                    name: net.interface,
                    type: 'ethernet', // Default, not provided by monitor status
                    status: 'up', // Default, not provided by monitor status
                    speed: null, // Not provided by monitor status
                    mtu: null, // Not provided by monitor status
                    ipAddresses: [], // Not provided by monitor status
                    macAddress: null, // Not provided by monitor status
                    rxBytes: net.bytesReceived,
                    txBytes: net.bytesSent,
                    rxPackets: net.packetsReceived,
                    txPackets: net.packetsSent,
                    rxErrors: net.dropsIn || 0,
                    txErrors: 0 // Not provided by monitor status
                })) || [],
                totalRxBytes: monitorData.networks?.reduce((total, net) => total + (net.bytesReceived || 0), 0),
                totalTxBytes: monitorData.networks?.reduce((total, net) => total + (net.bytesSent || 0), 0)
            },
            processesInfo: {
                processes: [
                    ...(monitorData.processes?.topByCPU || []),
                    ...(monitorData.processes?.topByMemory || [])
                ].map(proc => ({
                    pid: proc.pid,
                    name: proc.name,
                    command: proc.command,
                    user: 'N/A', // Not provided by monitor status
                    cpu: proc.cpuPercent || 0,
                    memory: proc.memoryPercent || 0,
                    memoryBytes: proc.memoryBytes || 0,
                    status: proc.status || 'unknown',
                    startTime: proc.startTime,
                    threads: null // Not provided by monitor status
                })).filter((proc, index, self) =>
                    // Remove duplicates based on PID
                    index === self.findIndex(p => p.pid === proc.pid)
                ),
                totalProcesses: monitorData.processes?.totalProcesses
            }
        };

        res.json(systemInfo);
    } catch (error) {
        console.error('Failed to get system info:', error);
        res.status(500).json({
            error: 'Failed to get system info',
            message: error.message
        });
    }
});

// Monitor system metrics
app.get('/api/monitor', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['monitor', 'status', '--json'], {
            node,
            maxBuffer: 1024 * 1024 * 10 // 10MB buffer instead of default 1MB
        });

        let metrics;
        try {
            const monitorData = JSON.parse(output);

            // Transform the monitor status data to match the expected frontend format
            metrics = {
                timestamp: monitorData.timestamp || new Date().toISOString(),
                cpu: {
                    cores: monitorData.cpu?.cores || 0,
                    usage: monitorData.cpu?.usagePercent || 0,
                    loadAverage: monitorData.cpu?.loadAverage || [0, 0, 0]
                },
                memory: {
                    total: monitorData.memory?.totalBytes || 0,
                    used: monitorData.memory?.usedBytes || 0,
                    available: monitorData.memory?.availableBytes || 0,
                    percent: monitorData.memory?.usagePercent || 0
                },
                disk: {
                    readBps: 0,  // Not provided by monitor status, would need monitor top
                    writeBps: 0, // Not provided by monitor status, would need monitor top
                    iops: 0      // Not provided by monitor status, would need monitor top
                },
                jobs: {
                    total: 0,    // Would need to get from jobs list
                    running: 0,  // Would need to get from jobs list
                    completed: 0,// Would need to get from jobs list
                    failed: 0    // Would need to get from jobs list
                }
            };
        } catch (e) {
            // Return basic metrics if command fails
            console.warn('Failed to parse monitor data:', e.message);
            metrics = {
                timestamp: new Date().toISOString(),
                cpu: {cores: 0, usage: 0, loadAverage: [0, 0, 0]},
                memory: {total: 0, used: 0, available: 0, percent: 0},
                disk: {readBps: 0, writeBps: 0, iops: 0},
                jobs: {total: 0, running: 0, completed: 0, failed: 0},
                error: 'Monitor command not available'
            };
        }

        res.json(metrics);
    } catch (error) {
        console.error('Failed to get monitor data:', error);

        // Return fallback metrics if monitoring fails
        const fallbackMetrics = {
            timestamp: new Date().toISOString(),
            cpu: {cores: 0, usage: 0, loadAverage: [0, 0, 0]},
            memory: {total: 0, used: 0, available: 0, percent: 0},
            disk: {readBps: 0, writeBps: 0, iops: 0},
            jobs: {total: 0, running: 0, completed: 0, failed: 0}
        };

        res.status(500).json({
            error: 'Failed to get monitor data',
            message: error.message,
            fallback: fallbackMetrics
        });
    }
});

// List volumes
app.get('/api/volumes', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['volume', 'list', '--json'], {node});

        let result;
        if (output && output.trim()) {
            try {
                result = JSON.parse(output);
                // Ensure volumes field exists and is an array
                if (!result.volumes || !Array.isArray(result.volumes)) {
                    result = {volumes: [], message: 'No volumes found'};
                }
            } catch (e) {
                console.warn('Failed to parse JSON from volume list:', e.message);
                result = {volumes: [], message: 'Volume service not available'};
            }
        } else {
            result = {volumes: [], message: 'Volume service not available'};
        }

        res.json(result);
    } catch (error) {
        console.error('Failed to list volumes:', error);
        res.json({volumes: [], message: `Volume service not available: ${error.message}`});
    }
});

// Delete volume
app.delete('/api/volumes/:volumeName', async (req, res) => {
    try {
        const {volumeName} = req.params;
        const node = req.query.node;

        if (!volumeName) {
            return res.status(400).json({error: 'Volume name is required'});
        }

        const output = await execRnx(['volume', 'remove', volumeName], {node});

        res.json({
            success: true,
            message: `Volume ${volumeName} deleted successfully`,
            output: output
        });
    } catch (error) {
        console.error(`Failed to delete volume ${req.params.volumeName}:`, error);
        res.status(500).json({
            error: 'Failed to delete volume',
            message: error.message
        });
    }
});

// List networks
app.get('/api/networks', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['network', 'list', '--json'], {node});

        let result;
        if (output && output.trim()) {
            try {
                result = JSON.parse(output);
                // Ensure networks field exists and is an array
                if (!result.networks || !Array.isArray(result.networks)) {
                    result = {
                        networks: [
                            {id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16'},
                            {id: 'host', name: 'host', type: 'host', subnet: ''}
                        ],
                        message: 'Using default networks'
                    };
                }
            } catch (e) {
                console.warn('Failed to parse JSON from network list:', e.message);
                result = {
                    networks: [
                        {id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16'},
                        {id: 'host', name: 'host', type: 'host', subnet: ''}
                    ],
                    message: 'Network service not available, showing defaults'
                };
            }
        } else {
            result = {
                networks: [
                    {id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16'},
                    {id: 'host', name: 'host', type: 'host', subnet: ''}
                ],
                message: 'Network service not available, showing defaults'
            };
        }

        res.json(result);
    } catch (error) {
        console.error('Failed to list networks:', error);
        res.json({
            networks: [
                {id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16'},
                {id: 'host', name: 'host', type: 'host', subnet: ''}
            ],
            message: `Network service not available: ${error.message}`
        });
    }
});

// Create a new volume
app.post('/api/volumes', async (req, res) => {
    try {
        const {name, size, type = 'filesystem'} = req.body;
        const node = req.query.node;

        if (!name || !size) {
            return res.status(400).json({error: 'Name and size are required'});
        }

        // Build rnx volume create command
        const args = ['volume', 'create', name, '--size', size];
        if (type !== 'filesystem') {
            args.push('--type', type);
        }

        const output = await execRnx(args, {node});

        res.json({
            success: true,
            message: `Volume ${name} created successfully`,
            name,
            size,
            type,
            output: output
        });
    } catch (error) {
        console.error('Failed to create volume:', error);
        res.status(500).json({
            error: 'Failed to create volume',
            message: error.message
        });
    }
});

// Create a new network
app.post('/api/networks', async (req, res) => {
    try {
        const {name, cidr} = req.body;
        const node = req.query.node;

        if (!name || !cidr) {
            return res.status(400).json({error: 'Name and CIDR are required'});
        }

        // Build rnx network create command
        const args = ['network', 'create', name, '--cidr', cidr];

        const output = await execRnx(args, {node});

        res.json({
            success: true,
            message: `Network ${name} created successfully`,
            name,
            cidr,
            output: output
        });
    } catch (error) {
        console.error('Failed to create network:', error);
        res.status(500).json({
            error: 'Failed to create network',
            message: error.message
        });
    }
});

// Delete network
app.delete('/api/networks/:networkName', async (req, res) => {
    try {
        const {networkName} = req.params;
        const node = req.query.node;

        if (!networkName) {
            return res.status(400).json({error: 'Network name is required'});
        }

        const output = await execRnx(['network', 'remove', networkName], {node});

        res.json({
            success: true,
            message: `Network ${networkName} deleted successfully`,
            output: output
        });
    } catch (error) {
        console.error(`Failed to delete network ${req.params.networkName}:`, error);
        res.status(500).json({
            error: 'Failed to delete network',
            message: error.message
        });
    }
});

// Validate workflow dependencies
app.post('/api/workflows/validate', async (req, res) => {
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

// Execute a workflow from file path
app.post('/api/workflows/execute', async (req, res) => {
    try {
        const {filePath, createMissingVolumes = false} = req.body;
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
                // Parse YAML to extract volume dependencies
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

                const allRequiredVolumes = Array.from(volumeSet);

                // Check which volumes exist and create missing ones
                if (allRequiredVolumes.length > 0) {
                    try {
                        const volumesOutput = await execRnx(['volume', 'list', '--json'], {node});
                        let existingVolumes = [];

                        if (volumesOutput && volumesOutput.trim()) {
                            const volumeData = JSON.parse(volumesOutput);
                            existingVolumes = volumeData.volumes || [];
                        }

                        const existingVolumeNames = existingVolumes.map(v => v.name);
                        const missingVolumes = allRequiredVolumes.filter(vol => !existingVolumeNames.includes(vol));

                        // Create missing volumes with default size
                        for (const volumeName of missingVolumes) {
                            try {
                                await execRnx(['volume', 'create', volumeName, '--size', '1GB'], {node});
                                console.log(`Created volume: ${volumeName}`);
                            } catch (createError) {
                                console.warn(`Failed to create volume ${volumeName}:`, createError.message);
                            }
                        }
                    } catch (volumeError) {
                        console.warn('Volume management failed:', volumeError.message);
                    }
                }
            } catch (parseError) {
                console.warn('Failed to parse workflow for volume creation:', parseError.message);
            }
        }

        try {
            // Execute the workflow directly from the file path
            const output = await execRnx(['run', '--workflow', filePath], {node});

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

// List workflows
app.get('/api/workflows', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['list', '--workflow', '--json'], {node});

        let workflows = [];
        if (output && output.trim()) {
            try {
                workflows = JSON.parse(output);
                // Ensure workflows is an array
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
        // Return empty array instead of error to prevent UI crashes
        res.json([]);
    }
});

// List runtimes
app.get('/api/runtimes', async (req, res) => {
    try {
        const node = req.query.node;

        // Get runtime list output (no --json flag available)
        const output = await execRnx(['runtime', 'list'], {node});

        let result;
        if (output && output.trim()) {
            // Parse text output
            const lines = output.split('\n').filter(line => line.trim());
            const runtimes = [];

            // Skip header lines (first 2 lines are header and separator)
            for (let i = 2; i < lines.length; i++) {
                const line = lines[i].trim();
                if (line && !line.startsWith('Use \'rnx runtime info')) {
                    const parts = line.split(/\s+/);
                    if (parts.length >= 4) {
                        runtimes.push({
                            id: parts[0],
                            name: parts[0],
                            version: parts[1],
                            size: parts[2],
                            description: parts.slice(3).join(' ')
                        });
                    }
                }
            }

            result = {runtimes};
        } else {
            result = {runtimes: [], message: 'Runtime service not available'};
        }

        res.json(result);
    } catch (error) {
        console.error('Failed to list runtimes:', error);
        res.json({runtimes: [], message: `Runtime service not available: ${error.message}`});
    }
});

// WebSocket handling
wss.on('connection', (ws, req) => {
    const url = new URL(req.url, `http://${req.headers.host}`);
    const pathname = url.pathname;

    console.log(`WebSocket connection established: ${pathname}`);

    if (pathname.startsWith('/ws/logs/')) {
        // Job log streaming
        const jobId = pathname.replace('/ws/logs/', '');
        handleLogStream(ws, jobId);
    } else if (pathname === '/ws/monitor') {
        // Monitor streaming
        handleMonitorStream(ws);
    } else {
        ws.close(1000, 'Unknown WebSocket endpoint');
    }
});

function handleLogStream(ws, jobId) {
    ws.send(JSON.stringify({
        type: 'connection',
        message: `Connected to log stream for job ${jobId}`,
        jobId: jobId,
        time: new Date().toISOString()
    }));

    // Start streaming logs using rnx log --follow
    const logProcess = spawn(RNX_PATH, ['log', jobId, '--follow'], {
        stdio: ['pipe', 'pipe', 'pipe']
    });

    let isAlive = true;

    // Stream stdout (logs)
    logProcess.stdout.on('data', (data) => {
        if (!isAlive) return;

        const logLines = data.toString().split('\n').filter(line => line.trim());
        logLines.forEach(line => {
            ws.send(JSON.stringify({
                type: 'log',
                jobId: jobId,
                message: line,
                time: new Date().toISOString()
            }));
        });
    });

    // Stream stderr (errors)
    logProcess.stderr.on('data', (data) => {
        if (!isAlive) return;

        const errorLines = data.toString().split('\n').filter(line => line.trim());
        errorLines.forEach(line => {
            ws.send(JSON.stringify({
                type: 'error',
                jobId: jobId,
                message: line,
                time: new Date().toISOString()
            }));
        });
    });

    // Handle process exit
    logProcess.on('close', (code) => {
        if (!isAlive) return;

        ws.send(JSON.stringify({
            type: 'status',
            jobId: jobId,
            message: `Log stream ended (exit code: ${code})`,
            time: new Date().toISOString()
        }));

        isAlive = false;
    });

    // Handle process error
    logProcess.on('error', (error) => {
        if (!isAlive) return;

        ws.send(JSON.stringify({
            type: 'error',
            jobId: jobId,
            message: `Log stream error: ${error.message}`,
            time: new Date().toISOString()
        }));

        isAlive = false;
    });

    // Clean up when WebSocket closes
    ws.on('close', () => {
        isAlive = false;
        if (logProcess && !logProcess.killed) {
            logProcess.kill('SIGTERM');
        }
    });

    // Handle WebSocket errors
    ws.on('error', (error) => {
        console.error('WebSocket error:', error);
        isAlive = false;
        if (logProcess && !logProcess.killed) {
            logProcess.kill('SIGTERM');
        }
    });
}

function handleMonitorStream(ws, node) {
    const interval = setInterval(async () => {
        try {
            const output = await execRnx(['monitor'], {node});
            const metrics = JSON.parse(output);

            ws.send(JSON.stringify({
                type: 'metrics',
                data: metrics,
                time: new Date().toISOString()
            }));
        } catch (error) {
            ws.send(JSON.stringify({
                type: 'error',
                message: `Monitor command failed: ${error.message}`,
                time: new Date().toISOString()
            }));
        }
    }, 5000);

    ws.on('close', () => {
        clearInterval(interval);
    });
}

// Serve React app for all other routes (SPA routing)
app.get('*', (req, res) => {
    res.sendFile(path.join(__dirname, '../ui/dist/index.html'));
});

// Start server
server.listen(PORT, BIND_ADDRESS, () => {
    console.log(`🚀 Joblet Admin Server running at http://${BIND_ADDRESS}:${PORT}`);
    console.log(`📡 API endpoints available at /api/*`);
    console.log(`🔌 WebSocket endpoints available at /ws/*`);
    console.log(`🎨 Admin UI served from /`);
});