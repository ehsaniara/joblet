import express from 'express';
import {execRnx} from '../utils/rnxExecutor.js';

const router = express.Router();

// Fetch comprehensive system information
router.get('/system-info', async (req, res) => {
    try {
        const node = req.query.node;

        // Ask the monitor for detailed system stats
        const output = await execRnx(['monitor', 'status', '--json'], {
            node,
            maxBuffer: 1024 * 1024 * 10 // 10MB buffer
        });

        const monitorData = JSON.parse(output);

        // Reorganize the data to match what the frontend expects
        const systemInfo = {
            hostInfo: {
                hostname: monitorData.hostInfo?.hostname,
                platform: monitorData.hostInfo?.platform,
                arch: monitorData.hostInfo?.arch,
                release: monitorData.hostInfo?.release,
                uptime: monitorData.hostInfo?.uptime,
                cloudProvider: monitorData.hostInfo?.cloudProvider,
                instanceType: monitorData.hostInfo?.instanceType,
                region: monitorData.hostInfo?.region,
                nodeId: monitorData.hostInfo?.nodeId,
                serverIPs: monitorData.hostInfo?.serverIPs,
                macAddresses: monitorData.hostInfo?.macAddresses
            },
            cpuInfo: {
                cores: monitorData.cpuInfo?.cores,
                threads: monitorData.cpuInfo?.threads,
                model: monitorData.cpuInfo?.model,
                frequency: monitorData.cpuInfo?.frequency,
                usage: monitorData.cpuInfo?.usage,
                loadAverage: monitorData.cpuInfo?.loadAverage,
                perCoreUsage: monitorData.cpuInfo?.perCoreUsage,
                temperature: monitorData.cpuInfo?.temperature
            },
            memoryInfo: {
                total: monitorData.memoryInfo?.total,
                used: monitorData.memoryInfo?.used,
                available: monitorData.memoryInfo?.available,
                percent: monitorData.memoryInfo?.percent,
                buffers: monitorData.memoryInfo?.buffers,
                cached: monitorData.memoryInfo?.cached,
                swap: monitorData.memoryInfo?.swap
            },
            disksInfo: {
                disks: monitorData.disksInfo?.disks?.map(disk => ({
                    name: disk.name,
                    mountpoint: disk.mountpoint,
                    filesystem: disk.filesystem,
                    size: disk.size,
                    used: disk.used,
                    available: disk.available,
                    percent: disk.percent,
                    readBps: disk.readBps,
                    writeBps: disk.writeBps,
                    iops: disk.iops
                })) || [],
                totalSpace: monitorData.disksInfo?.totalSpace,
                usedSpace: monitorData.disksInfo?.usedSpace
            },
            networkInfo: {
                interfaces: monitorData.networkInfo?.interfaces?.map(iface => ({
                    name: iface.name,
                    type: iface.type || 'ethernet',
                    status: iface.status || iface.state || 'unknown',
                    speed: iface.speed || iface.link_speed,
                    mtu: iface.mtu,
                    ipAddresses: iface.ipAddresses || iface.ip_addresses || iface.addresses || (iface.ipv4 ? [iface.ipv4] : []),
                    macAddress: iface.macAddress || iface.mac_address || iface.mac || iface.hwaddr,
                    rxBytes: iface.rxBytes || iface.rx_bytes || iface.bytes_recv,
                    txBytes: iface.txBytes || iface.tx_bytes || iface.bytes_sent,
                    rxPackets: iface.rxPackets || iface.rx_packets || iface.packets_recv,
                    txPackets: iface.txPackets || iface.tx_packets || iface.packets_sent,
                    rxErrors: iface.rxErrors || iface.rx_errors || iface.errin,
                    txErrors: iface.txErrors || iface.tx_errors || iface.errout
                })) || [],
                totalRxBytes: monitorData.networkInfo?.totalRxBytes,
                totalTxBytes: monitorData.networkInfo?.totalTxBytes
            },
            processesInfo: {
                processes: monitorData.processesInfo?.processes || [],
                totalProcesses: monitorData.processesInfo?.totalProcesses
            }
        };

        res.json(systemInfo);
    } catch (error) {
        console.error('Failed to get system info:', error);
        res.status(500).json({error: error.message});
    }
});

// Fetch system performance metrics
router.get('/system-metrics', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['monitor', 'status', '--json'], {node});
        const metrics = JSON.parse(output);
        res.json(metrics);
    } catch (error) {
        console.error('Failed to get system metrics:', error);
        res.status(500).json({error: error.message});
    }
});

// Get volumes
router.get('/volumes', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['volume', 'list', '--json'], {node});

        let result;
        if (output && output.trim()) {
            try {
                result = JSON.parse(output);
                // Make sure we have a proper volumes list
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

// Create volume
router.post('/volumes', async (req, res) => {
    try {
        const {name, size, type} = req.body;
        const node = req.query.node;

        if (!name) {
            return res.status(400).json({error: 'Volume name is required'});
        }

        const args = ['volume', 'create', name];
        if (size) args.push('--size', size);
        if (type) args.push('--type', type);

        const output = await execRnx(args, {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to create volume:', error);
        res.status(500).json({error: error.message});
    }
});

// Delete volume
router.delete('/volumes/:volumeName', async (req, res) => {
    try {
        const {volumeName} = req.params;
        const node = req.query.node;

        const output = await execRnx(['volume', 'remove', volumeName], {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to remove volume:', error);
        res.status(500).json({error: error.message});
    }
});

// Get networks
router.get('/networks', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['network', 'list', '--json'], {node});

        let result;
        if (output && output.trim()) {
            try {
                result = JSON.parse(output);
                // Make sure we have a proper networks list
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

// Create network
router.post('/networks', async (req, res) => {
    try {
        const {name, cidr, type} = req.body;
        const node = req.query.node;

        if (!name) {
            return res.status(400).json({error: 'Network name is required'});
        }

        if (!cidr) {
            return res.status(400).json({error: 'CIDR range is required'});
        }

        const args = ['network', 'create', name];
        args.push('--cidr', cidr);
        if (type) args.push('--type', type);

        const output = await execRnx(args, {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to create network:', error);
        res.status(500).json({error: error.message});
    }
});

// Delete network
router.delete('/networks/:networkName', async (req, res) => {
    try {
        const {networkName} = req.params;
        const node = req.query.node;

        const output = await execRnx(['network', 'remove', networkName], {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to delete network:', error);
        res.status(500).json({error: error.message});
    }
});

// Get runtimes
router.get('/runtimes', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['runtime', 'list', '--json'], {node});

        let runtimes = [];
        if (output && output.trim()) {
            try {
                const runtimeData = JSON.parse(output);
                // The RNX command gives us an array, not an object with a runtimes field
                const rawRuntimes = Array.isArray(runtimeData) ? runtimeData : (runtimeData.runtimes || []);

                // Reshape the data for the frontend
                runtimes = rawRuntimes.map(runtime => ({
                    id: runtime.id,
                    name: runtime.name || runtime.id,
                    version: runtime.version,
                    size: runtime.size || `${Math.round(runtime.size_bytes / 1024 / 1024)}MB`,
                    description: runtime.description || `${runtime.language} runtime`
                }));
            } catch (e) {
                console.warn('Failed to parse runtime JSON:', e.message);
                runtimes = [];
            }
        }

        res.json({runtimes});
    } catch (error) {
        console.error('Failed to list runtimes:', error);
        res.json({runtimes: []});
    }
});

// Install runtime
router.post('/runtimes/install', async (req, res) => {
    try {
        const {name, force} = req.body;
        const node = req.query.node;

        if (!name) {
            return res.status(400).json({error: 'Runtime name is required'});
        }

        const args = ['runtime', 'install', name, '--github-repo=ehsaniara/joblet/tree/main/runtimes'];
        if (force) {
            args.push('--force');
        }

        const output = await execRnx(args, {node});

        // Look for the job ID in the command output
        let buildJobId = null;
        const lines = output.split('\n');
        for (const line of lines) {
            if (line.includes('Build Job:')) {
                buildJobId = line.split('Build Job:')[1]?.trim();
                break;
            }
        }

        res.json({
            success: true,
            output: output,
            buildJobId: buildJobId
        });
    } catch (error) {
        console.error('Failed to install runtime:', error);
        res.status(500).json({error: error.message});
    }
});

// Remove runtime
router.delete('/runtimes/:runtimeName', async (req, res) => {
    try {
        const {runtimeName} = req.params;
        const node = req.query.node;

        const output = await execRnx(['runtime', 'remove', runtimeName], {node});
        res.json({success: true, output});
    } catch (error) {
        console.error('Failed to remove runtime:', error);
        res.status(500).json({error: error.message});
    }
});

// Get monitor data (legacy endpoint)
router.get('/monitor', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['monitor', 'status', '--json'], {node});
        const metrics = JSON.parse(output);
        res.json(metrics);
    } catch (error) {
        console.error('Failed to get monitor data:', error);
        res.status(500).json({error: error.message});
    }
});

// Cache for GitHub runtime data to avoid repeated API calls
let githubRuntimesCache = null;
let githubCacheTimestamp = 0;
const CACHE_DURATION = 5 * 60 * 1000; // Keep cache for 5 minutes

router.get('/github/runtimes', async (req, res) => {
    try {
        // Use the repository path from the request, or default to the main joblet repo
        const repoPath = req.query.repo || 'ehsaniara/joblet/tree/main/runtimes';

        // If we already have fresh data for this repo, just return it
        const cacheKey = `${repoPath}`;
        const now = Date.now();
        if (githubRuntimesCache &&
            githubRuntimesCache.repo === cacheKey &&
            (now - githubCacheTimestamp < CACHE_DURATION)) {
            return res.json(githubRuntimesCache.data);
        }

        // Get the runtime list directly from GitHub since the RNX command has issues
        console.log('Fetching runtime manifest directly from GitHub...');

        // Break down the repo path to build the URL for the manifest file
        const pathParts = repoPath.split('/');
        let owner, repo, branch = 'main', path = '';

        if (pathParts.length >= 2) {
            owner = pathParts[0];
            repo = pathParts[1];

            // Handle the full GitHub URL format with branch and path
            if (pathParts.length >= 4 && pathParts[2] === 'tree') {
                branch = pathParts[3];
                path = pathParts.slice(4).join('/');
            }
        } else {
            throw new Error('Invalid repository format. Expected: owner/repo or owner/repo/tree/branch/path');
        }

        const manifestUrl = `https://raw.githubusercontent.com/${owner}/${repo}/${branch}/${path}/runtime-manifest.json`;

        const fetch = (await import('node-fetch')).default;
        const response = await fetch(manifestUrl);

        if (!response.ok) {
            throw new Error(`Failed to fetch manifest: ${response.statusText}`);
        }

        const manifest = await response.json();

        // Convert the GitHub manifest data into the format our UI expects
        const runtimesFromManifest = Object.entries(manifest.runtimes || {}).map(([name, runtime]) => {
            // The manifest stores platforms as an array of strings like ['ubuntu-amd64', 'ubuntu-arm64']
            let platforms = [];
            if (Array.isArray(runtime.platforms)) {
                // Change the format from "ubuntu-amd64" to "ubuntu/amd64" for display
                platforms = runtime.platforms.map(p => p.replace('-', '/'));
            }

            // Build the GitHub link that users can click to view the runtime
            const githubUrl = path
                ? `https://github.com/${owner}/${repo}/tree/${branch}/${path}/${name}`
                : `https://github.com/${owner}/${repo}/tree/${branch}/${name}`;

            return {
                name: name,
                type: 'directory',
                html_url: githubUrl,
                platforms: platforms,
                displayName: runtime.display_name || name,
                description: runtime.description,
                category: runtime.category,
                language: runtime.language,
                version: runtime.version,
                requirements: runtime.requirements,
                provides: runtime.provides,
                tags: runtime.tags
            };
        });

        // Save this data so we don't have to fetch it again soon
        githubRuntimesCache = {
            repo: cacheKey,
            data: runtimesFromManifest
        };
        githubCacheTimestamp = now;

        res.json(runtimesFromManifest);
    } catch (error) {
        console.error('Failed to fetch GitHub runtimes:', error);

        // If something went wrong, let the frontend know what happened
        res.status(500).json({
            error: 'Failed to fetch runtimes from GitHub',
            message: error.message,
            runtimes: []
        });
    }
});

// Get version information
router.get('/version', async (req, res) => {
    try {
        const node = req.query.node;
        const output = await execRnx(['--version'], {node});

        // Parse the version output
        // Expected format: "rnx v4.4.0+dev (1614267)\njoblet v4.4.0+dev (298124f)"
        const lines = output.split('\n');
        const rnxLine = lines.find(line => line.startsWith('rnx '));
        const jobletLine = lines.find(line => line.startsWith('joblet '));

        const versionInfo = {};

        if (rnxLine) {
            const match = rnxLine.match(/rnx\s+(.+)/);
            if (match) {
                versionInfo.rnx = match[1];
            }
        }

        if (jobletLine) {
            const match = jobletLine.match(/joblet\s+(.+)/);
            if (match) {
                versionInfo.joblet = match[1];
            }
        }

        res.json(versionInfo);
    } catch (error) {
        console.error('Failed to get version info:', error);
        res.status(500).json({error: error.message});
    }
});

export default router;