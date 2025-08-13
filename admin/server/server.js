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
    const command = `${RNX_PATH} ${args.join(' ')}`;
    console.log(`Executing: ${command}`);
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

// List all jobs
app.get('/api/jobs', async (req, res) => {
  try {
    const output = await execRnx(['list', '--json']);
    
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
      name,
      workdir
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
    if (name) rnxArgs.push('--name', name);
    if (workdir) rnxArgs.push('--workdir', workdir);
    if (schedule) rnxArgs.push('--schedule', schedule);
    
    // Add volumes
    volumes.forEach(volume => {
      rnxArgs.push('--volume', volume);
    });

    // Execute the job
    const output = await execRnx(rnxArgs);
    
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
    const output = await execRnx(['status', jobId, '--json']);
    
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

// Monitor system metrics
app.get('/api/monitor', async (req, res) => {
  try {
    const output = await execRnx(['monitor']);
    
    let metrics;
    try {
      metrics = JSON.parse(output);
    } catch (e) {
      // Return basic metrics if command fails
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
    res.status(500).json({ error: 'Failed to get monitor data', message: error.message });
  }
});

// List volumes
app.get('/api/volumes', async (req, res) => {
  try {
    const output = await execRnx(['volume', 'list', '--json']);
    
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

// List networks
app.get('/api/networks', async (req, res) => {
  try {
    const output = await execRnx(['network', 'list', '--json']);
    
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

function handleMonitorStream(ws) {
  const interval = setInterval(async () => {
    try {
      const output = await execRnx(['monitor']);
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