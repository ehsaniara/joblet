import express from 'express';
import {execRnx} from '../utils/rnxExecutor.js';

const router = express.Router();

// Get detailed system info
router.get('/system-info', async (req, res) => {
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
                hostname: monitorData.hostInfo?.hostname,
                platform: monitorData.hostInfo?.platform,
                arch: monitorData.hostInfo?.arch,
                release: monitorData.hostInfo?.release,
                uptime: monitorData.hostInfo?.uptime,
                cloudProvider: monitorData.hostInfo?.cloudProvider,
                instanceType: monitorData.hostInfo?.instanceType,
                region: monitorData.hostInfo?.region
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
                interfaces: monitorData.networkInfo?.interfaces || [],
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

// Get system metrics
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
                // rnx runtime list returns an array directly, not an object with runtimes property
                const rawRuntimes = Array.isArray(runtimeData) ? runtimeData : (runtimeData.runtimes || []);

                // Transform to match UI expectations
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

        // Extract build job ID from output
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

// RNX native GitHub runtime listing with caching
let githubRuntimesCache = null;
let githubCacheTimestamp = 0;
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

router.get('/github/runtimes', async (req, res) => {
    try {
        // Check if we have valid cached data
        const now = Date.now();
        if (githubRuntimesCache && (now - githubCacheTimestamp < CACHE_DURATION)) {
            return res.json(githubRuntimesCache);
        }

        // Use RNX native GitHub runtime listing
        const node = req.query.node;
        const output = await execRnx(['runtime', 'list', '--github-repo=ehsaniara/joblet/tree/main/runtimes', '--json'], {node});

        // Extract JSON from the output (skip status messages)
        const lines = output.split('\n');
        let jsonStart = -1;
        for (let i = 0; i < lines.length; i++) {
            if (lines[i].trim().startsWith('[')) {
                jsonStart = i;
                break;
            }
        }

        const jsonOutput = jsonStart >= 0 ? lines.slice(jsonStart).join('\n') : output;
        const runtimeData = JSON.parse(jsonOutput);

        // Transform RNX runtime data to match UI expectations
        const runtimesWithPlatforms = runtimeData.map(runtime => {
            // Extract platform list from the platforms object
            const platforms = Object.keys(runtime.platforms || {})
                .filter(platform => runtime.platforms[platform].supported)
                .map(platform => platform.replace('-', '/'))
                .sort();

            return {
                name: runtime.name,
                type: 'directory',
                html_url: `https://github.com/ehsaniara/joblet/tree/main/runtimes/${runtime.name}`,
                platforms: platforms,
                displayName: runtime.display_name || runtime.name,
                description: runtime.description,
                category: runtime.category,
                language: runtime.language,
                version: runtime.version,
                requirements: runtime.requirements,
                provides: runtime.provides,
                tags: runtime.tags
            };
        });

        // Cache the result
        githubRuntimesCache = runtimesWithPlatforms;
        githubCacheTimestamp = now;

        res.json(runtimesWithPlatforms);
    } catch (error) {
        console.error('Failed to fetch GitHub runtimes via RNX:', error);

        // Return fallback static data when RNX GitHub listing is unavailable
        const fallbackRuntimes = [
            {
                name: 'graalvmjdk-21',
                type: 'directory',
                html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/graalvmjdk-21',
                platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64'],
                displayName: 'GraalVM JDK 21',
                description: 'GraalVM Community Edition JDK 21 with native-image support',
                category: 'language-runtime',
                language: 'java'
            },
            {
                name: 'openjdk-21',
                type: 'directory',
                html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/openjdk-21',
                platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64'],
                displayName: 'OpenJDK 21',
                description: 'OpenJDK 21 with Java development tools',
                category: 'language-runtime',
                language: 'java'
            },
            {
                name: 'python-3.11-ml',
                type: 'directory',
                html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/python-3.11-ml',
                platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64'],
                displayName: 'Python 3.11 ML',
                description: 'Python 3.11 with machine learning libraries',
                category: 'language-runtime',
                language: 'python'
            }
        ];

        // Cache the fallback data
        githubRuntimesCache = fallbackRuntimes;
        githubCacheTimestamp = Date.now();

        res.json(fallbackRuntimes);
    }
});

export default router;