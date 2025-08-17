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
import { exec, spawn } from 'child_process';
import { promisify } from 'util';
import { WebSocketServer } from 'ws';
import { createServer } from 'http';
import path from 'path';
import { fileURLToPath } from 'url';
import cors from 'cors';

const execAsync = promisify(exec);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();
const server = createServer(app);
const wss = new WebSocketServer({ server });

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
    const { stdout, stderr } = await execAsync(command, options);
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
      nodes.unshift({ name: 'default', status: 'active', default: true });
    }
    
    res.json(nodes);
  } catch (error) {
    console.error('Failed to list nodes:', error);
    // Return default node on error
    res.json([{ name: 'default', status: 'active', default: true }]);
  }
});

// List all jobs
app.get('/api/jobs', async (req, res) => {
  try {
    const node = req.query.node;
    const output = await execRnx(['list', '--json'], { node });
    
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
      schedule,
      workdir,
      node
    } = req.body;

    if (!command) {
      return res.status(400).json({ error: 'Command is required' });
    }

    // Build rnx run command arguments
    const rnxArgs = ['run', command, ...args];
    
    // Add optional flags
    if (maxCPU) rnxArgs.push('--max-cpu', maxCPU.toString());
    if (maxMemory) rnxArgs.push('--max-memory', maxMemory.toString());
    if (maxIOBPS) rnxArgs.push('--max-iobps', maxIOBPS.toString());
    if (cpuCores) rnxArgs.push('--cpu-cores', cpuCores);
    if (runtime) rnxArgs.push('--runtime', runtime);
    if (network) rnxArgs.push('--network', network);
    if (workdir) rnxArgs.push('--workdir', workdir);
    if (schedule) rnxArgs.push('--schedule', schedule);
    
    // Add volumes
    volumes.forEach(volume => {
      rnxArgs.push('--volume', volume);
    });

    // Execute the job
    const output = await execRnx(rnxArgs, { node });
    
    // Extract job ID from output (adjust based on rnx run output format)
    const jobId = output || `job-${Date.now()}`;
    
    res.json({
      jobId,
      status: 'created',
      message: 'Job created successfully'
    });
  } catch (error) {
    console.error('Failed to execute job:', error);
    res.status(500).json({ error: 'Failed to execute job', message: error.message });
  }
});

// Get job details
app.get('/api/jobs/:jobId', async (req, res) => {
  try {
    const { jobId } = req.params;
    const node = req.query.node;
    const output = await execRnx(['status', jobId, '--json'], { node });
    
    let jobDetails;
    if (output && output.trim()) {
      try {
        jobDetails = JSON.parse(output);
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
      id: jobId,
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
        cpu: { cores: 0, usage: 0, loadAverage: [0, 0, 0] },
        memory: { total: 0, used: 0, available: 0, percent: 0 },
        disk: { readBps: 0, writeBps: 0, iops: 0 },
        jobs: { total: 0, running: 0, completed: 0, failed: 0 },
        error: 'Monitor command not available'
      };
    }
    
    res.json(metrics);
  } catch (error) {
    console.error('Failed to get monitor data:', error);
    
    // Return fallback metrics if monitoring fails
    const fallbackMetrics = {
      timestamp: new Date().toISOString(),
      cpu: { cores: 0, usage: 0, loadAverage: [0, 0, 0] },
      memory: { total: 0, used: 0, available: 0, percent: 0 },
      disk: { readBps: 0, writeBps: 0, iops: 0 },
      jobs: { total: 0, running: 0, completed: 0, failed: 0 }
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
    const output = await execRnx(['volume', 'list', '--json'], { node });
    
    let result;
    if (output && output.trim()) {
      try {
        result = JSON.parse(output);
        // Ensure volumes field exists and is an array
        if (!result.volumes || !Array.isArray(result.volumes)) {
          result = { volumes: [], message: 'No volumes found' };
        }
      } catch (e) {
        console.warn('Failed to parse JSON from volume list:', e.message);
        result = { volumes: [], message: 'Volume service not available' };
      }
    } else {
      result = { volumes: [], message: 'Volume service not available' };
    }
    
    res.json(result);
  } catch (error) {
    console.error('Failed to list volumes:', error);
    res.json({ volumes: [], message: `Volume service not available: ${error.message}` });
  }
});

// Delete volume
app.delete('/api/volumes/:volumeName', async (req, res) => {
  try {
    const { volumeName } = req.params;
    const node = req.query.node;
    
    if (!volumeName) {
      return res.status(400).json({ error: 'Volume name is required' });
    }
    
    const output = await execRnx(['volume', 'remove', volumeName], { node });
    
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
    const output = await execRnx(['network', 'list', '--json'], { node });
    
    let result;
    if (output && output.trim()) {
      try {
        result = JSON.parse(output);
        // Ensure networks field exists and is an array
        if (!result.networks || !Array.isArray(result.networks)) {
          result = {
            networks: [
              { id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16' },
              { id: 'host', name: 'host', type: 'host', subnet: '' }
            ],
            message: 'Using default networks'
          };
        }
      } catch (e) {
        console.warn('Failed to parse JSON from network list:', e.message);
        result = {
          networks: [
            { id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16' },
            { id: 'host', name: 'host', type: 'host', subnet: '' }
          ],
          message: 'Network service not available, showing defaults'
        };
      }
    } else {
      result = {
        networks: [
          { id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16' },
          { id: 'host', name: 'host', type: 'host', subnet: '' }
        ],
        message: 'Network service not available, showing defaults'
      };
    }
    
    res.json(result);
  } catch (error) {
    console.error('Failed to list networks:', error);
    res.json({
      networks: [
        { id: 'bridge', name: 'bridge', type: 'bridge', subnet: '172.17.0.0/16' },
        { id: 'host', name: 'host', type: 'host', subnet: '' }
      ],
      message: `Network service not available: ${error.message}`
    });
  }
});

// List runtimes
app.get('/api/runtimes', async (req, res) => {
  try {
    const node = req.query.node;
    // Try with --json first, fallback to parsing text output
    let output;
    let useJson = true;
    
    try {
      output = await execRnx(['runtime', 'list', '--json'], { node });
    } catch (error) {
      // If --json flag fails, try without it
      if (error.message.includes('unknown flag')) {
        useJson = false;
        output = await execRnx(['runtime', 'list'], { node });
      } else {
        throw error;
      }
    }
    
    let result;
    if (output && output.trim()) {
      if (useJson) {
        try {
          result = JSON.parse(output);
          // Ensure runtimes field exists and is an array
          if (!result.runtimes || !Array.isArray(result.runtimes)) {
            result = { runtimes: [], message: 'No runtimes found' };
          }
        } catch (e) {
          console.warn('Failed to parse JSON from runtime list:', e.message);
          result = { runtimes: [], message: 'Runtime service not available' };
        }
      } else {
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
        
        result = { runtimes };
      }
    } else {
      result = { runtimes: [], message: 'Runtime service not available' };
    }
    
    res.json(result);
  } catch (error) {
    console.error('Failed to list runtimes:', error);
    res.json({ runtimes: [], message: `Runtime service not available: ${error.message}` });
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
      const output = await execRnx(['monitor'], { node });
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