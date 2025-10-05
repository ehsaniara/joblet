import {spawn} from 'child_process';
import {execRnx} from '../utils/rnxExecutor.js';
import {config} from '../config.js';

export function handleLogStream(ws, jobId) {
    let isAlive = true;
    let followingLogProcess = null;

    ws.send(JSON.stringify({
        type: 'connection',
        message: `Connected to log stream for job ${jobId}`,
        jobId: jobId,
        time: new Date().toISOString()
    }));

    // First, try to get historical logs without --follow
    const historicalLogProcess = spawn(config.RNX_PATH, ['job', 'log', jobId], {
        stdio: ['pipe', 'pipe', 'pipe']
    });

    // Get historical logs first
    historicalLogProcess.stdout.on('data', (data) => {
        if (!isAlive) return;

        const logLines = data.toString().split('\n').filter(line => line.trim());
        logLines.forEach(line => {
            // Detect if this is a joblet system log and extract log level
            // INFO logs don't have [component], others do: [timestamp] [LEVEL] [component] vs [timestamp] [INFO] message
            const jobletLogMatch = line.match(/^\[.*?\] \[(DEBUG|INFO|WARNING|ERROR)\]/);

            let subtype = 'output';
            if (jobletLogMatch) {
                const logLevel = jobletLogMatch[1];
                subtype = logLevel === 'INFO' ? 'info' : 'system';
            }

            ws.send(JSON.stringify({
                type: 'log',
                subtype: subtype,
                jobId: jobId,
                message: line,
                time: new Date().toISOString()
            }));
        });
    });

    historicalLogProcess.stderr.on('data', (data) => {
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

    historicalLogProcess.on('close', (code) => {
        if (!isAlive) return;

        // Check if job is still running by trying to get status
        execRnx(['job', 'status', jobId, '--json'], {node: 'default'})
            .then(output => {
                if (!isAlive) return;

                try {
                    const jobData = JSON.parse(output);
                    const isRunning = jobData.status === 'RUNNING';

                    if (isRunning) {
                        // Only start following if job is actually running
                        startFollowingLogs();
                    } else {
                        // Job is completed, just send completion message
                        ws.send(JSON.stringify({
                            type: 'connection',
                            message: `Historical logs loaded for ${jobData.status.toLowerCase()} job ${jobId}`,
                            jobId: jobId,
                            time: new Date().toISOString()
                        }));
                    }
                } catch (e) {
                    // If we can't parse status, assume job is completed
                    ws.send(JSON.stringify({
                        type: 'connection',
                        message: `Historical logs loaded for job ${jobId}`,
                        jobId: jobId,
                        time: new Date().toISOString()
                    }));
                }
            })
            .catch(() => {
                // If status check fails, assume job is completed
                if (!isAlive) return;
                ws.send(JSON.stringify({
                    type: 'connection',
                    message: `Historical logs loaded for job ${jobId}`,
                    jobId: jobId,
                    time: new Date().toISOString()
                }));
            });
    });

    function startFollowingLogs() {
        followingLogProcess = spawn(config.RNX_PATH, ['job', 'log', jobId, '--follow'], {
            stdio: ['pipe', 'pipe', 'pipe']
        });

        // Stream new logs
        followingLogProcess.stdout.on('data', (data) => {
            if (!isAlive) return;

            const logLines = data.toString().split('\n').filter(line => line.trim());
            logLines.forEach(line => {
                // Detect if this is a joblet system log and extract log level
                const jobletLogMatch = line.match(/^\[.*?\] \[(DEBUG|INFO|WARNING|ERROR)\]/);

                let subtype = 'output';
                if (jobletLogMatch) {
                    const logLevel = jobletLogMatch[1];
                    subtype = logLevel === 'INFO' ? 'info' : 'system';
                }

                ws.send(JSON.stringify({
                    type: 'log',
                    subtype: subtype,
                    jobId: jobId,
                    message: line,
                    time: new Date().toISOString()
                }));
            });
        });

        followingLogProcess.stderr.on('data', (data) => {
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

        followingLogProcess.on('close', (followCode) => {
            if (!isAlive) return;

            ws.send(JSON.stringify({
                type: 'connection',
                message: `Log stream for job ${jobId} ended`,
                jobId: jobId,
                time: new Date().toISOString()
            }));
        });
    }

    // Cleanup on WebSocket close
    ws.on('close', () => {
        isAlive = false;
        if (historicalLogProcess && !historicalLogProcess.killed) {
            historicalLogProcess.kill();
        }
        if (followingLogProcess && !followingLogProcess.killed) {
            followingLogProcess.kill();
        }
    });
}

export function handleWorkflowStatusStream(ws, workflowId, node) {
    const interval = setInterval(async () => {
        try {
            const output = await execRnx(['job', 'list', '--json'], {node});
            const jobs = JSON.parse(output);

            // Find jobs that belong to this workflow
            const workflowJobs = jobs.filter(job => job.workflowId === workflowId);

            ws.send(JSON.stringify({
                type: 'workflow_status_change',
                workflowId: workflowId,
                jobs: workflowJobs,
                time: new Date().toISOString()
            }));
        } catch (error) {
            ws.send(JSON.stringify({
                type: 'error',
                message: `Workflow status update failed: ${error.message}`,
                time: new Date().toISOString()
            }));
        }
    }, 3000);

    ws.on('close', () => {
        clearInterval(interval);
    });
}

export function handleRuntimeInstallStream(ws, buildJobId, node) {
    let isAlive = true;
    let followingLogProcess = null;

    ws.send(JSON.stringify({
        type: 'connection',
        message: `Connected to runtime build stream for job ${buildJobId}`,
        buildJobId: buildJobId,
        time: new Date().toISOString()
    }));

    // Start following the build logs immediately
    followingLogProcess = spawn(config.RNX_PATH, ['job', 'log', buildJobId, '--follow'], {
        stdio: ['pipe', 'pipe', 'pipe']
    });

    followingLogProcess.stdout.on('data', (data) => {
        if (!isAlive) return;

        const logLines = data.toString().split('\n').filter(line => line.trim());
        logLines.forEach(line => {
            // Detect if this is a joblet system log and extract log level
            const jobletLogMatch = line.match(/^\[.*?\] \[(DEBUG|INFO|WARNING|ERROR)\]/);

            let subtype = 'output';
            if (jobletLogMatch) {
                const logLevel = jobletLogMatch[1];
                subtype = logLevel === 'INFO' ? 'info' : 'system';
            }

            ws.send(JSON.stringify({
                type: 'log',
                subtype: subtype,
                buildJobId: buildJobId,
                message: line,
                time: new Date().toISOString()
            }));
        });
    });

    followingLogProcess.stderr.on('data', (data) => {
        if (!isAlive) return;

        const errorLines = data.toString().split('\n').filter(line => line.trim());
        errorLines.forEach(line => {
            ws.send(JSON.stringify({
                type: 'error',
                buildJobId: buildJobId,
                message: line,
                time: new Date().toISOString()
            }));
        });
    });

    followingLogProcess.on('close', (code) => {
        if (!isAlive) return;

        // Check final status of the build job
        execRnx(['job', 'status', buildJobId, '--json'], {node})
            .then(output => {
                if (!isAlive) return;

                try {
                    const jobData = JSON.parse(output);
                    const isCompleted = jobData.status === 'COMPLETED';
                    const isFailed = jobData.status === 'FAILED';

                    ws.send(JSON.stringify({
                        type: isCompleted ? 'completed' : isFailed ? 'failed' : 'ended',
                        buildJobId: buildJobId,
                        status: jobData.status,
                        message: `Runtime build ${jobData.status.toLowerCase()}`,
                        exitCode: jobData.exitCode,
                        time: new Date().toISOString()
                    }));
                } catch (e) {
                    ws.send(JSON.stringify({
                        type: 'ended',
                        buildJobId: buildJobId,
                        message: 'Runtime build process ended',
                        time: new Date().toISOString()
                    }));
                }
            })
            .catch(() => {
                if (!isAlive) return;
                ws.send(JSON.stringify({
                    type: 'ended',
                    buildJobId: buildJobId,
                    message: 'Runtime build process ended',
                    time: new Date().toISOString()
                }));
            });
    });

    // Cleanup on WebSocket close
    ws.on('close', () => {
        isAlive = false;
        if (followingLogProcess && !followingLogProcess.killed) {
            followingLogProcess.kill();
        }
    });
}

export function handleJobMetricsStream(ws, jobId) {
    let isAlive = true;
    let metricsProcess = null;
    let buffer = '';

    ws.send(JSON.stringify({
        type: 'connection',
        message: `Connected to metrics stream for job ${jobId}`,
        jobId: jobId,
        time: new Date().toISOString()
    }));

    // Start the metrics streaming process - this will show historical data first,
    // then continue with live streaming for running jobs
    metricsProcess = spawn(config.RNX_PATH, ['job', 'metrics', jobId, '--json'], {
        stdio: ['pipe', 'pipe', 'pipe']
    });

    metricsProcess.stdout.on('data', (data) => {
        if (!isAlive) return;

        buffer += data.toString();

        // Try to parse complete JSON objects from the buffer
        let braceCount = 0;
        let jsonStart = -1;

        for (let i = 0; i < buffer.length; i++) {
            const char = buffer[i];

            if (char === '{') {
                if (braceCount === 0) {
                    jsonStart = i;
                }
                braceCount++;
            } else if (char === '}') {
                braceCount--;

                if (braceCount === 0 && jsonStart >= 0) {
                    // Found complete JSON object
                    const jsonStr = buffer.substring(jsonStart, i + 1);

                    try {
                        const metricData = JSON.parse(jsonStr);
                        ws.send(JSON.stringify({
                            type: 'metrics',
                            jobId: jobId,
                            data: metricData,
                            time: new Date().toISOString()
                        }));
                    } catch (e) {
                        // Skip invalid JSON
                        console.warn('Failed to parse metrics JSON:', e.message);
                    }

                    // Remove processed JSON from buffer
                    buffer = buffer.substring(i + 1);
                    i = -1; // Reset loop
                    jsonStart = -1;
                }
            }
        }
    });

    metricsProcess.stderr.on('data', (data) => {
        if (!isAlive) return;

        const errorOutput = data.toString();
        console.error('Metrics stream error for job', jobId, ':', errorOutput);

        // Check if it's the "no metrics available" error
        if (errorOutput.includes('no metrics available')) {
            ws.send(JSON.stringify({
                type: 'error',
                jobId: jobId,
                message: 'No metrics available for this job. Metrics collection may not be enabled.',
                time: new Date().toISOString()
            }));
        } else {
            // Send other errors
            const errorLines = errorOutput.split('\n').filter(line => line.trim());
            errorLines.forEach(line => {
                ws.send(JSON.stringify({
                    type: 'error',
                    jobId: jobId,
                    message: line,
                    time: new Date().toISOString()
                }));
            });
        }
    });

    metricsProcess.on('close', (code) => {
        if (!isAlive) return;

        ws.send(JSON.stringify({
            type: 'connection',
            message: `Metrics stream for job ${jobId} ended (exit code: ${code})`,
            jobId: jobId,
            time: new Date().toISOString()
        }));
    });

    // Cleanup on WebSocket close
    ws.on('close', () => {
        isAlive = false;
        if (metricsProcess && !metricsProcess.killed) {
            metricsProcess.kill();
        }
    });
}

export function handleMonitorStream(ws, node) {
    const interval = setInterval(async () => {
        try {
            const output = await execRnx(['monitor', 'status', '--json'], {node});
            const monitorData = JSON.parse(output);

            // Transform the rnx monitor data to match the SystemMetrics interface
            const metrics = {
                timestamp: new Date().toISOString(),
                available: true,
                cpu: {
                    cores: monitorData.cpuInfo?.cores || 0,
                    usagePercent: monitorData.cpuInfo?.usage || 0,
                    loadAverage: monitorData.cpuInfo?.loadAverage || [0, 0, 0],
                    perCoreUsage: monitorData.cpuInfo?.perCoreUsage || []
                },
                memory: {
                    totalBytes: monitorData.memoryInfo?.total || 0,
                    usedBytes: monitorData.memoryInfo?.used || 0,
                    availableBytes: monitorData.memoryInfo?.available || 0,
                    usagePercent: monitorData.memoryInfo?.percent || 0,
                    cachedBytes: monitorData.memoryInfo?.cached || 0,
                    bufferedBytes: monitorData.memoryInfo?.buffers || 0
                },
                disks: (monitorData.disksInfo?.disks || []).map(disk => ({
                    device: disk.name,
                    mountPoint: disk.mountpoint,
                    filesystem: disk.filesystem,
                    totalBytes: disk.size,
                    usedBytes: disk.used,
                    freeBytes: disk.available,
                    usagePercent: disk.percent
                })),
                processes: {
                    totalProcesses: monitorData.processesInfo?.totalProcesses || 0,
                    totalThreads: 0 // Not available in current rnx output
                }
            };

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