import {useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Cpu, Download, ExternalLink, HardDrive, Info, Network, Plus, RefreshCw, Trash2, X} from 'lucide-react';
import {apiService} from '../services/apiService';

interface Volume {
    id?: string;
    name: string;
    size: string;
    type: string;
    created_time?: string;
    mountPath?: string;
}

interface NetworkResource {
    id: string;
    name: string;
    type: string;
    subnet: string;
}

interface Runtime {
    id: string;
    name: string;
    version: string;
    size: string;
    description: string;
}

interface GitHubRuntime {
    name: string;
    type: string;
    download_url?: string;
    html_url?: string;
    platforms?: string[];
    isInstalled?: boolean;
    displayName?: string;
    description?: string;
    category?: string;
    language?: string;
    version?: string;
    requirements?: {
        min_ram_mb: number;
        min_disk_mb: number;
        gpu_required: boolean;
    };
    provides?: {
        executables: string[];
        libraries?: string[];
        environment_vars?: Record<string, string>;
    };
    tags?: string[];
}

const Resources: React.FC = () => {
    const {t} = useTranslation();
    const [volumes, setVolumes] = useState<Volume[]>([]);
    const [networks, setNetworks] = useState<NetworkResource[]>([]);
    const [runtimes, setRuntimes] = useState<Runtime[]>([]);
    const [loading, setLoading] = useState({
        volumes: true,
        networks: true,
        runtimes: true
    });
    const [error, setError] = useState({
        volumes: '',
        networks: '',
        runtimes: ''
    });
    const [deleteConfirm, setDeleteConfirm] = useState<{
        show: boolean;
        volumeName: string;
        deleting: boolean;
    }>({
        show: false,
        volumeName: '',
        deleting: false
    });

    const [deleteNetworkConfirm, setDeleteNetworkConfirm] = useState<{
        show: boolean;
        networkName: string;
        deleting: boolean;
    }>({
        show: false,
        networkName: '',
        deleting: false
    });

    const [createVolumeModal, setCreateVolumeModal] = useState({
        show: false,
        creating: false
    });

    const [createNetworkModal, setCreateNetworkModal] = useState({
        show: false,
        creating: false
    });

    const [runtimesDialog, setRuntimesDialog] = useState({
        show: false,
        loading: false,
        error: '',
        runtimes: [] as GitHubRuntime[],
        repository: 'ehsaniara/joblet/tree/main/runtimes',
        validatingRepo: false,
        searchQuery: ''
    });

    const [installProgress, setInstallProgress] = useState({
        show: false,
        runtimeName: '',
        buildJobId: '',
        logs: [] as string[],
        status: 'building' as 'building' | 'completed' | 'failed' | 'error'
    });

    const [runtimeConfirm, setRuntimeConfirm] = useState({
        show: false,
        action: 'install' as 'install' | 'reinstall' | 'remove',
        runtimeName: '',
        processing: false
    });

    const logsEndRef = useRef<HTMLDivElement>(null);

    const [volumeForm, setVolumeForm] = useState({
        name: '',
        size: '',
        type: 'filesystem'
    });

    const [volumeFormErrors, setVolumeFormErrors] = useState({
        name: '',
        size: ''
    });

    const [networkForm, setNetworkForm] = useState({
        name: '',
        cidr: ''
    });

    const fetchVolumes = async () => {
        try {
            setLoading(prev => ({...prev, volumes: true}));
            setError(prev => ({...prev, volumes: ''}));
            const response = await apiService.getVolumes();
            setVolumes(response.volumes || []);
        } catch (err) {
            setError(prev => ({...prev, volumes: err instanceof Error ? err.message : 'Failed to fetch volumes'}));
        } finally {
            setLoading(prev => ({...prev, volumes: false}));
        }
    };

    const fetchNetworks = async () => {
        try {
            setLoading(prev => ({...prev, networks: true}));
            setError(prev => ({...prev, networks: ''}));
            const response = await apiService.getNetworks();
            setNetworks(response.networks || []);
        } catch (err) {
            setError(prev => ({...prev, networks: err instanceof Error ? err.message : 'Failed to fetch networks'}));
        } finally {
            setLoading(prev => ({...prev, networks: false}));
        }
    };

    const fetchRuntimes = async () => {
        try {
            setLoading(prev => ({...prev, runtimes: true}));
            setError(prev => ({...prev, runtimes: ''}));
            const response = await apiService.getRuntimes();
            setRuntimes(response.runtimes || []);
        } catch (err) {
            setError(prev => ({...prev, runtimes: err instanceof Error ? err.message : 'Failed to fetch runtimes'}));
        } finally {
            setLoading(prev => ({...prev, runtimes: false}));
        }
    };

    // Function to validate if repository has runtime-manifest.json
    const validateRuntimeRepository = async (repoPath: string): Promise<boolean> => {
        try {
            // Parse repository path: "owner/repo/tree/branch/path" 
            const pathParts = repoPath.split('/');
            if (pathParts.length < 5) return false;

            const owner = pathParts[0];
            const repo = pathParts[1];
            const branch = pathParts[3]; // skip "tree"
            const path = pathParts.slice(4).join('/');

            // Check for runtime-manifest.json in the specified path
            const manifestUrl = `https://api.github.com/repos/${owner}/${repo}/contents/${path}/runtime-manifest.json?ref=${branch}`;
            const response = await fetch(manifestUrl);

            return response.ok;
        } catch {
            return false;
        }
    };

    const fetchGitHubRuntimes = async (customRepo?: string) => {
        const repoPath = customRepo || runtimesDialog.repository;
        setRuntimesDialog(prev => ({...prev, loading: true, error: ''}));

        try {
            // First validate that the repository has runtime-manifest.json
            const isValid = await validateRuntimeRepository(repoPath);
            if (!isValid) {
                throw new Error(t('resources.repositoryNoManifest'));
            }

            // Parse repository path: "owner/repo/tree/branch/path"
            const pathParts = repoPath.split('/');
            const owner = pathParts[0];
            const repo = pathParts[1];
            const branch = pathParts[3]; // skip "tree"
            const path = pathParts.slice(4).join('/');

            // Try server-side proxy first to avoid rate limits (if using default repo)
            let response;
            if (repoPath === 'ehsaniara/joblet/tree/main/runtimes') {
                response = await fetch('/api/github/runtimes');

                // Fallback to direct GitHub API if proxy doesn't exist
                if (!response.ok && response.status === 404) {
                    response = await fetch(`https://api.github.com/repos/${owner}/${repo}/contents/${path}?ref=${branch}`);
                }
            } else {
                // For custom repositories, always use direct GitHub API
                response = await fetch(`https://api.github.com/repos/${owner}/${repo}/contents/${path}?ref=${branch}`);
            }

            if (!response.ok) {
                if (response.status === 403) {
                    throw new Error('GitHub API rate limit exceeded. Please try again later or use a personal access token.');
                }
                throw new Error(`GitHub API error: ${response.status} - ${response.statusText}`);
            }

            const data = await response.json();

            // Handle server proxy response (already processed) vs direct GitHub API response
            let runtimesWithPlatforms;
            if (Array.isArray(data) && data.length > 0 && data[0].platforms) {
                // Server proxy response - already has platforms
                runtimesWithPlatforms = data;
            } else if (Array.isArray(data)) {
                // Direct GitHub API response - need to process
                const runtimeDirs = data.filter(item => item.type === 'dir');

                runtimesWithPlatforms = await Promise.all(
                    runtimeDirs.map(async (runtime) => {
                        try {
                            const platformResponse = await fetch(`https://api.github.com/repos/${owner}/${repo}/contents/${path}/${runtime.name}?ref=${branch}`);

                            if (platformResponse.ok) {
                                const platformData = await platformResponse.json();

                                if (Array.isArray(platformData)) {
                                    const setupFiles = platformData.filter(item =>
                                        item.type === 'file' &&
                                        item.name.startsWith('setup-') &&
                                        item.name.endsWith('.sh')
                                    );

                                    const platforms = new Set<string>();

                                    setupFiles.forEach(file => {
                                        const match = file.name.match(/setup-([^-]+)-([^.]+)\.sh/);
                                        if (match) {
                                            const [, os, arch] = match;
                                            platforms.add(`${os}/${arch}`);
                                        }
                                    });

                                    return {
                                        name: runtime.name,
                                        type: 'directory',
                                        html_url: runtime.html_url,
                                        platforms: Array.from(platforms).sort()
                                    };
                                }
                            }

                            return {
                                name: runtime.name,
                                type: 'directory',
                                html_url: runtime.html_url,
                                platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64']
                            };
                        } catch {
                            return {
                                name: runtime.name,
                                type: 'directory',
                                html_url: runtime.html_url,
                                platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64']
                            };
                        }
                    })
                );
            } else {
                throw new Error('Invalid response format');
            }

            // Compare with local runtimes to mark installed status
            const localRuntimeNames = runtimes.map(r => r.name.toLowerCase());
            const runtimesWithInstallStatus = runtimesWithPlatforms.map(runtime => ({
                ...runtime,
                isInstalled: localRuntimeNames.includes(runtime.name.toLowerCase())
            }));

            setRuntimesDialog(prev => ({
                ...prev,
                runtimes: runtimesWithInstallStatus,
                loading: false
            }));
        } catch (err) {
            // Fallback to known runtimes if GitHub API fails
            const fallbackRuntimes = [
                {
                    name: 'graalvmjdk-21',
                    type: 'directory',
                    html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/graalvmjdk-21',
                    platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64']
                },
                {
                    name: 'openjdk-21',
                    type: 'directory',
                    html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/openjdk-21',
                    platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64']
                },
                {
                    name: 'python-3.11-ml',
                    type: 'directory',
                    html_url: 'https://github.com/ehsaniara/joblet/tree/main/runtimes/python-3.11-ml',
                    platforms: ['ubuntu/amd64', 'ubuntu/arm64', 'rhel/amd64', 'rhel/arm64', 'amzn/amd64', 'amzn/arm64']
                }
            ];

            // Compare with local runtimes for install status
            const localRuntimeNames = runtimes.map(r => r.name.toLowerCase());
            const fallbackWithStatus = fallbackRuntimes.map(runtime => ({
                ...runtime,
                isInstalled: localRuntimeNames.includes(runtime.name.toLowerCase())
            }));

            setRuntimesDialog(prev => ({
                ...prev,
                runtimes: fallbackWithStatus,
                error: 'GitHub API temporarily unavailable - showing known runtimes',
                loading: false
            }));
        }
    };

    const openRuntimesDialog = async () => {
        setRuntimesDialog(prev => ({...prev, show: true}));

        // Fetch both local and GitHub runtimes
        await Promise.all([
            fetchRuntimes(), // Refresh local runtimes
            fetchGitHubRuntimes()
        ]);
    };

    const closeRuntimesDialog = () => {
        setRuntimesDialog({
            show: false,
            loading: false,
            error: '',
            runtimes: [],
            repository: 'ehsaniara/joblet/tree/main/runtimes',
            validatingRepo: false,
            searchQuery: ''
        });
    };

    const performInstallRuntime = async (runtimeName: string, force: boolean = false) => {
        try {
            const response = await fetch('/api/runtimes/install', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({name: runtimeName, force: force})
            });

            if (!response.ok) {
                throw new Error(`Failed to install runtime: ${response.status}`);
            }

            const result = await response.json();

            if (result.buildJobId) {
                // Open streaming progress dialog
                setInstallProgress({
                    show: true,
                    runtimeName: runtimeName,
                    buildJobId: result.buildJobId,
                    logs: [result.output || 'Starting runtime installation...'],
                    status: 'building'
                });

                // Connect to WebSocket for real-time logs
                connectToInstallStream(result.buildJobId);
            } else {
                // Fallback to old behavior if no build job ID
                await fetchRuntimes();
                await fetchGitHubRuntimes();
            }
        } catch (error) {
            setRuntimesDialog(prev => ({
                ...prev,
                error: error instanceof Error ? error.message : 'Failed to install runtime'
            }));
        }
    };

    const connectToInstallStream = (buildJobId: string) => {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/runtime-install/${buildJobId}`;
        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
            console.log('Connected to runtime install stream');
        };

        ws.onmessage = (event) => {
            const message = JSON.parse(event.data);

            setInstallProgress(prev => {
                const newLogs = [...prev.logs];

                if (message.type === 'log') {
                    newLogs.push(message.message);
                } else if (message.type === 'error') {
                    newLogs.push(`ERROR: ${message.message}`);
                } else if (message.type === 'completed') {
                    newLogs.push('✅ Runtime installation completed successfully!');
                    return {
                        ...prev,
                        logs: newLogs,
                        status: 'completed'
                    };
                } else if (message.type === 'failed') {
                    newLogs.push('❌ Runtime installation failed!');
                    return {
                        ...prev,
                        logs: newLogs,
                        status: 'failed'
                    };
                } else if (message.type === 'connection') {
                    newLogs.push(message.message);
                }

                return {
                    ...prev,
                    logs: newLogs
                };
            });

            // Auto-scroll to bottom
            setTimeout(() => {
                logsEndRef.current?.scrollIntoView({behavior: 'smooth'});
            }, 100);
        };

        ws.onclose = () => {
            console.log('Runtime install stream closed');
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            setInstallProgress(prev => ({
                ...prev,
                logs: [...prev.logs, 'Connection error occurred'],
                status: 'error'
            }));
        };
    };

    const closeInstallProgress = async () => {
        setInstallProgress({
            show: false,
            runtimeName: '',
            buildJobId: '',
            logs: [],
            status: 'building'
        });

        // Refresh both local runtimes and dialog to update install status
        await fetchRuntimes();
        await fetchGitHubRuntimes();
    };

    const showRuntimeConfirmation = (action: 'install' | 'reinstall' | 'remove', runtimeName: string) => {
        setRuntimeConfirm({
            show: true,
            action: action,
            runtimeName: runtimeName,
            processing: false
        });
    };

    const cancelRuntimeAction = () => {
        setRuntimeConfirm({
            show: false,
            action: 'install',
            runtimeName: '',
            processing: false
        });
    };

    const confirmRuntimeAction = async () => {
        setRuntimeConfirm(prev => ({...prev, processing: true}));

        try {
            if (runtimeConfirm.action === 'remove') {
                await performRemoveRuntime(runtimeConfirm.runtimeName);
            } else {
                const isReinstall = runtimeConfirm.action === 'reinstall';
                await performInstallRuntime(runtimeConfirm.runtimeName, isReinstall);
            }

            setRuntimeConfirm({
                show: false,
                action: 'install',
                runtimeName: '',
                processing: false
            });
        } catch (error) {
            setRuntimeConfirm(prev => ({...prev, processing: false}));
            setRuntimesDialog(prev => ({
                ...prev,
                error: error instanceof Error ? error.message : 'Operation failed'
            }));
        }
    };

    const performRemoveRuntime = async (runtimeName: string) => {
        try {
            const response = await fetch(`/api/runtimes/${runtimeName}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                throw new Error(`Failed to remove runtime: ${response.status}`);
            }

            // Refresh both local runtimes and dialog to update install status
            await fetchRuntimes();
            await fetchGitHubRuntimes();
        } catch (error) {
            setRuntimesDialog(prev => ({
                ...prev,
                error: error instanceof Error ? error.message : 'Failed to remove runtime'
            }));
        }
    };

    const refreshAll = () => {
        fetchVolumes();
        fetchNetworks();
        fetchRuntimes();
    };

    const handleDeleteVolume = async (volumeName: string) => {
        setDeleteConfirm({show: true, volumeName, deleting: false});
    };

    const handleDeleteNetwork = (networkName: string) => {
        setDeleteNetworkConfirm({show: true, networkName, deleting: false});
    };

    const confirmDeleteNetwork = async () => {
        if (!deleteNetworkConfirm.networkName) return;

        setDeleteNetworkConfirm(prev => ({...prev, deleting: true}));

        try {
            await apiService.deleteNetwork(deleteNetworkConfirm.networkName);
            setDeleteNetworkConfirm({show: false, networkName: '', deleting: false});
            await fetchNetworks(); // Refresh the network list
        } catch (error) {
            console.error('Failed to delete network:', error);
            setError(prev => ({
                ...prev,
                networks: error instanceof Error ? error.message : 'Failed to delete network'
            }));
            setDeleteNetworkConfirm(prev => ({...prev, deleting: false}));
        }
    };

    const cancelDeleteNetwork = () => {
        setDeleteNetworkConfirm({show: false, networkName: '', deleting: false});
    };

    const confirmDeleteVolume = async () => {
        if (!deleteConfirm.volumeName) return;

        setDeleteConfirm(prev => ({...prev, deleting: true}));

        try {
            await apiService.deleteVolume(deleteConfirm.volumeName);
            setDeleteConfirm({show: false, volumeName: '', deleting: false});
            await fetchVolumes(); // Refresh the volume list
        } catch (error) {
            console.error('Failed to delete volume:', error);
            setError(prev => ({
                ...prev,
                volumes: error instanceof Error ? error.message : 'Failed to delete volume'
            }));
            setDeleteConfirm(prev => ({...prev, deleting: false}));
        }
    };

    const cancelDeleteVolume = () => {
        setDeleteConfirm({show: false, volumeName: '', deleting: false});
    };

    // Validation functions
    const validateVolumeName = (name: string): string => {
        if (!name) return 'Volume name is required';
        if (!/^[a-zA-Z0-9][a-zA-Z0-9-_]*$/.test(name)) {
            return 'Name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores';
        }
        if (name.length > 63) {
            return 'Name must be 63 characters or less';
        }
        return '';
    };

    const validateVolumeSize = (size: string): string => {
        if (!size) return 'Volume size is required';
        if (!/^\d+(\.\d+)?(B|KB|MB|GB|TB)$/i.test(size)) {
            return 'Size must be a number followed by unit (B, KB, MB, GB, TB). Example: 1GB, 500MB';
        }
        return '';
    };

    const handleVolumeNameChange = (name: string) => {
        setVolumeForm(prev => ({...prev, name}));
        setVolumeFormErrors(prev => ({...prev, name: validateVolumeName(name)}));
    };

    const handleVolumeSizeChange = (size: string) => {
        setVolumeForm(prev => ({...prev, size}));
        setVolumeFormErrors(prev => ({...prev, size: validateVolumeSize(size)}));
    };

    const handleCreateVolume = async () => {
        // Validate all fields
        const nameError = validateVolumeName(volumeForm.name);
        const sizeError = validateVolumeSize(volumeForm.size);

        if (nameError || sizeError) {
            setVolumeFormErrors({
                name: nameError,
                size: sizeError
            });
            return;
        }

        setCreateVolumeModal(prev => ({...prev, creating: true}));

        try {
            await apiService.createVolume(volumeForm.name, volumeForm.size, volumeForm.type);
            setCreateVolumeModal({show: false, creating: false});
            setVolumeForm({name: '', size: '', type: 'filesystem'});
            setVolumeFormErrors({name: '', size: ''});
            await fetchVolumes(); // Refresh the volume list
        } catch (error) {
            setError(prev => ({
                ...prev,
                volumes: error instanceof Error ? error.message : 'Failed to create volume'
            }));
            setCreateVolumeModal(prev => ({...prev, creating: false}));
        }
    };

    const handleCreateNetwork = async () => {
        if (!networkForm.name || !networkForm.cidr) return;

        setCreateNetworkModal(prev => ({...prev, creating: true}));

        try {
            await apiService.createNetwork(networkForm.name, networkForm.cidr);
            setCreateNetworkModal({show: false, creating: false});
            setNetworkForm({name: '', cidr: ''});
            await fetchNetworks(); // Refresh the network list
        } catch (error) {
            setError(prev => ({
                ...prev,
                networks: error instanceof Error ? error.message : 'Failed to create network'
            }));
            setCreateNetworkModal(prev => ({...prev, creating: false}));
        }
    };

    // Auto-refresh functionality
    // Resources are static, no auto-refresh needed - users can manually refresh via buttons

    useEffect(() => {
        refreshAll();
    }, []);

    const formatSize = (size: string | number): string => {
        // If it's already a formatted string with units, return as-is
        if (typeof size === 'string' && /\d+(\.\d+)?\s*(B|KB|MB|GB|TB)$/i.test(size)) {
            return size;
        }

        // Convert string to number if it's just a number
        const numericSize = typeof size === 'string' ? parseInt(size) : size;

        if (numericSize === 0 || isNaN(numericSize)) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(numericSize) / Math.log(k));
        return parseFloat((numericSize / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    // Filter runtimes based on search query
    const filteredRuntimes = runtimesDialog.runtimes.filter(runtime => {
        const searchLower = runtimesDialog.searchQuery.toLowerCase();
        return (
            runtime.name.toLowerCase().includes(searchLower) ||
            runtime.displayName?.toLowerCase().includes(searchLower) ||
            runtime.description?.toLowerCase().includes(searchLower) ||
            runtime.language?.toLowerCase().includes(searchLower) ||
            runtime.category?.toLowerCase().includes(searchLower)
        );
    });

    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">{t('resources.title')}</h1>
                        <p className="mt-2 text-gray-300">{t('resources.subtitle')}</p>
                    </div>
                    <button
                        onClick={refreshAll}
                        className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                    >
                        <RefreshCw className="h-4 w-4 mr-2"/>
                        Refresh All
                    </button>
                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Volumes */}
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="flex items-center">
                                <HardDrive className="h-6 w-6 text-blue-600 mr-3"/>
                                <h3 className="text-lg font-semibold text-gray-200">Volumes</h3>
                            </div>
                            <button
                                onClick={fetchVolumes}
                                className="text-gray-400 hover:text-gray-600"
                                title={t('resources.refreshVolumes')}
                            >
                                <RefreshCw className="h-4 w-4"/>
                            </button>
                        </div>

                        {loading.volumes ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500">{t('resources.loadingVolumes')}</p>
                            </div>
                        ) : error.volumes ? (
                            <div className="text-center py-8">
                                <p className="text-red-500 text-sm">{error.volumes}</p>
                            </div>
                        ) : volumes.length === 0 ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500 mb-4">No volumes configured</p>
                                <button
                                    onClick={() => setCreateVolumeModal({show: true, creating: false})}
                                    className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700">
                                    <Plus className="h-4 w-4 mr-2"/>
                                    Create Volume
                                </button>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {volumes.map((volume, index) => (
                                    <div key={volume.id || volume.name || index} className="border rounded-lg p-3">
                                        <div className="flex items-center justify-between">
                                            <div className="flex-1">
                                                <p className="font-medium text-gray-300">{volume.name}</p>
                                                <p className="text-sm text-gray-500">{volume.type}</p>
                                                <p className="text-sm text-gray-500">{volume.mountPath || `/volumes/${volume.name}`}</p>
                                            </div>
                                            <div className="text-right mr-3">
                                                <p className="text-sm text-gray-600">{formatSize(volume.size)}</p>
                                                {volume.created_time && (
                                                    <p className="text-xs text-gray-400">{new Date(volume.created_time).toLocaleDateString()}</p>
                                                )}
                                            </div>
                                            <div>
                                                <button
                                                    onClick={() => handleDeleteVolume(volume.name)}
                                                    className="text-red-400 hover:text-red-300 p-1 rounded transition-colors"
                                                    title={t('resources.deleteVolume')}
                                                >
                                                    <Trash2 className="h-4 w-4"/>
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                                <button
                                    onClick={() => setCreateVolumeModal({show: true, creating: false})}
                                    className="w-full mt-4 inline-flex items-center justify-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700">
                                    <Plus className="h-4 w-4 mr-2"/>
                                    Create Volume
                                </button>
                            </div>
                        )}
                    </div>
                </div>

                {/* Networks */}
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="flex items-center">
                                <Network className="h-6 w-6 text-green-600 mr-3"/>
                                <h3 className="text-lg font-semibold text-gray-200">Networks</h3>
                            </div>
                            <button
                                onClick={fetchNetworks}
                                className="text-gray-400 hover:text-gray-600"
                                title={t('resources.refreshNetworks')}
                            >
                                <RefreshCw className="h-4 w-4"/>
                            </button>
                        </div>

                        {loading.networks ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500">{t('resources.loadingNetworks')}</p>
                            </div>
                        ) : error.networks ? (
                            <div className="text-center py-8">
                                <p className="text-red-500 text-sm">{error.networks}</p>
                            </div>
                        ) : networks.length === 0 ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500 mb-4">No networks configured</p>
                                <button
                                    onClick={() => setCreateNetworkModal({show: true, creating: false})}
                                    className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-green-600 hover:bg-green-700">
                                    <Plus className="h-4 w-4 mr-2"/>
                                    Create Network
                                </button>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {networks.map((network, index) => (
                                    <div key={network.id || network.name || index} className="border rounded-lg p-3">
                                        <div className="flex items-center justify-between">
                                            <div>
                                                <p className="font-medium text-gray-300">{network.name}</p>
                                                <p className="text-sm text-gray-500">{network.type}</p>
                                            </div>
                                            <div className="text-right mr-3">
                                                <p className="text-sm text-gray-600">{network.subnet || 'N/A'}</p>
                                            </div>
                                            {(network.name !== 'bridge' && network.name !== 'host') && (
                                                <div>
                                                    <button
                                                        onClick={() => handleDeleteNetwork(network.name)}
                                                        className="text-red-400 hover:text-red-300 p-1 rounded transition-colors"
                                                        title={t('resources.deleteNetwork')}
                                                    >
                                                        <Trash2 className="h-4 w-4"/>
                                                    </button>
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                ))}
                                <button
                                    onClick={() => setCreateNetworkModal({show: true, creating: false})}
                                    className="w-full mt-4 inline-flex items-center justify-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-green-600 hover:bg-green-700">
                                    <Plus className="h-4 w-4 mr-2"/>
                                    Create Network
                                </button>
                            </div>
                        )}
                    </div>
                </div>

                {/* Runtimes */}
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="flex items-center">
                                <Cpu className="h-6 w-6 text-purple-600 mr-3"/>
                                <h3 className="text-lg font-semibold text-gray-200">Runtimes</h3>
                            </div>
                            <button
                                onClick={fetchRuntimes}
                                className="text-gray-400 hover:text-gray-600"
                                title={t('resources.refreshRuntimes')}
                            >
                                <RefreshCw className="h-4 w-4"/>
                            </button>
                        </div>

                        {loading.runtimes ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500">{t('resources.loadingRuntimes')}</p>
                            </div>
                        ) : error.runtimes ? (
                            <div className="text-center py-8">
                                <p className="text-red-500 text-sm">{error.runtimes}</p>
                            </div>
                        ) : runtimes.length === 0 ? (
                            <div className="text-center py-8">
                                <p className="text-gray-500 mb-4">No runtimes installed</p>
                                <button
                                    onClick={openRuntimesDialog}
                                    className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-purple-600 hover:bg-purple-700">
                                    <Info className="h-4 w-4 mr-2"/>
                                    View All Runtimes
                                </button>
                            </div>
                        ) : (
                            <div className="space-y-3">
                                {runtimes.map((runtime, index) => (
                                    <div key={runtime.id || runtime.name || index} className="border rounded-lg p-3">
                                        <div>
                                            <p className="font-medium text-gray-300">{runtime.name}</p>
                                            <p className="text-sm text-gray-500">{runtime.description}</p>
                                            <div className="flex items-center justify-between mt-2">
                                                <p className="text-xs text-gray-400">v{runtime.version}</p>
                                                <p className="text-xs text-gray-400">{runtime.size}</p>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                                <button
                                    onClick={openRuntimesDialog}
                                    className="w-full mt-4 inline-flex items-center justify-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50">
                                    <Info className="h-4 w-4 mr-2"/>
                                    View All Runtimes
                                </button>
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Delete Confirmation Dialog */}
            {deleteConfirm.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    Delete Volume
                                </h3>
                                <button
                                    onClick={cancelDeleteVolume}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={deleteConfirm.deleting}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        Are you sure you want to delete the volume "{deleteConfirm.volumeName}"?
                                    </p>
                                    <p className="text-sm text-red-400">
                                        This action cannot be undone. All data in this volume will be permanently lost.
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre
                                        className="bg-gray-900 text-red-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{`rnx volume remove ${deleteConfirm.volumeName}`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelDeleteVolume}
                                    disabled={deleteConfirm.deleting}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmDeleteVolume}
                                    disabled={deleteConfirm.deleting}
                                    className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {deleteConfirm.deleting ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Deleting...
                                        </>
                                    ) : (
                                        <>
                                            <Trash2 className="h-4 w-4 mr-2"/>
                                            Delete
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Create Volume Modal */}
            {createVolumeModal.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">Create Volume</h3>
                                <button
                                    onClick={() => {
                                        setCreateVolumeModal({show: false, creating: false});
                                        setVolumeFormErrors({name: '', size: ''});
                                    }}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={createVolumeModal.creating}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Name
                                    </label>
                                    <input
                                        type="text"
                                        value={volumeForm.name}
                                        onChange={(e) => handleVolumeNameChange(e.target.value)}
                                        className={`w-full px-3 py-2 border rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 ${
                                            volumeFormErrors.name
                                                ? 'border-red-500 focus:ring-red-500'
                                                : 'border-gray-600 focus:ring-blue-500'
                                        }`}
                                        placeholder="e.g., backend, cache, data"
                                        disabled={createVolumeModal.creating}
                                    />
                                    {volumeFormErrors.name && (
                                        <p className="mt-1 text-xs text-red-400">{volumeFormErrors.name}</p>
                                    )}
                                </div>

                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Size
                                        </label>
                                        <input
                                            type="text"
                                            value={volumeForm.size}
                                            onChange={(e) => handleVolumeSizeChange(e.target.value)}
                                            className={`w-full px-3 py-2 border rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 ${
                                                volumeFormErrors.size
                                                    ? 'border-red-500 focus:ring-red-500'
                                                    : 'border-gray-600 focus:ring-blue-500'
                                            }`}
                                            placeholder="e.g., 1GB, 500MB"
                                            disabled={createVolumeModal.creating}
                                        />
                                        {volumeFormErrors.size && (
                                            <p className="mt-1 text-xs text-red-400">{volumeFormErrors.size}</p>
                                        )}
                                    </div>

                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Type
                                        </label>
                                        <select
                                            value={volumeForm.type}
                                            onChange={(e) => setVolumeForm(prev => ({...prev, type: e.target.value}))}
                                            className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                            disabled={createVolumeModal.creating}
                                        >
                                            <option value="filesystem">Filesystem</option>
                                            <option value="memory">Memory (tmpfs)</option>
                                        </select>
                                    </div>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre
                                        className="bg-gray-900 text-green-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{volumeForm.name && volumeForm.size
    ? `rnx volume create ${volumeForm.name} --size=${volumeForm.size}${volumeForm.type !== 'filesystem' ? ` --type=${volumeForm.type}` : ''}`
    : '# Enter volume name and size to see command preview'}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={() => {
                                        setCreateVolumeModal({show: false, creating: false});
                                        setVolumeFormErrors({name: '', size: ''});
                                    }}
                                    disabled={createVolumeModal.creating}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={handleCreateVolume}
                                    disabled={createVolumeModal.creating || !volumeForm.name || !volumeForm.size || !!volumeFormErrors.name || !!volumeFormErrors.size}
                                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {createVolumeModal.creating ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Creating...
                                        </>
                                    ) : (
                                        <>
                                            <Plus className="h-4 w-4 mr-2"/>
                                            Create
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Delete Network Confirmation Dialog */}
            {deleteNetworkConfirm.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    Delete Network
                                </h3>
                                <button
                                    onClick={cancelDeleteNetwork}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={deleteNetworkConfirm.deleting}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        Are you sure you want to delete the network "{deleteNetworkConfirm.networkName}"?
                                    </p>
                                    <p className="text-sm text-red-400">
                                        This action cannot be undone. Any containers using this network must be stopped
                                        first.
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre
                                        className="bg-gray-900 text-red-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{`rnx network remove ${deleteNetworkConfirm.networkName}`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelDeleteNetwork}
                                    disabled={deleteNetworkConfirm.deleting}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmDeleteNetwork}
                                    disabled={deleteNetworkConfirm.deleting}
                                    className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {deleteNetworkConfirm.deleting ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Deleting...
                                        </>
                                    ) : (
                                        <>
                                            <Trash2 className="h-4 w-4 mr-2"/>
                                            Delete
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Create Network Modal */}
            {createNetworkModal.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">Create Network</h3>
                                <button
                                    onClick={() => setCreateNetworkModal({show: false, creating: false})}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={createNetworkModal.creating}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Name
                                    </label>
                                    <input
                                        type="text"
                                        value={networkForm.name}
                                        onChange={(e) => setNetworkForm(prev => ({...prev, name: e.target.value}))}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-green-500"
                                        placeholder="e.g., backend-net, app-network"
                                        disabled={createNetworkModal.creating}
                                    />
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        CIDR Range
                                    </label>
                                    <input
                                        type="text"
                                        value={networkForm.cidr}
                                        onChange={(e) => setNetworkForm(prev => ({...prev, cidr: e.target.value}))}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-gray-200 focus:outline-none focus:ring-2 focus:ring-green-500"
                                        placeholder="e.g., 10.1.0.0/24, 192.168.100.0/24"
                                        disabled={createNetworkModal.creating}
                                    />
                                    <p className="text-xs text-gray-400 mt-1">
                                        Format: IP address followed by subnet mask (e.g., 10.1.0.0/24)
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre
                                        className="bg-gray-900 text-green-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{networkForm.name && networkForm.cidr
    ? `rnx network create ${networkForm.name} --cidr=${networkForm.cidr}`
    : '# Enter network name and CIDR range to see command preview'}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={() => setCreateNetworkModal({show: false, creating: false})}
                                    disabled={createNetworkModal.creating}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={handleCreateNetwork}
                                    disabled={createNetworkModal.creating || !networkForm.name || !networkForm.cidr}
                                    className="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:bg-green-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {createNetworkModal.creating ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Creating...
                                        </>
                                    ) : (
                                        <>
                                            <Plus className="h-4 w-4 mr-2"/>
                                            Create
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* GitHub Runtimes Dialog */}
            {runtimesDialog.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div
                        className="relative bg-gray-800 rounded-lg shadow-xl max-w-5xl w-full mx-4 max-h-[95vh] overflow-hidden">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    {t('resources.runtimes')} from GitHub
                                </h3>
                                <button
                                    onClick={closeRuntimesDialog}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={runtimesDialog.loading}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            {/* Repository Input */}
                            <div className="mb-4">
                                <label className="block text-sm font-medium text-gray-300 mb-2">
                                    {t('resources.githubRepositoryPath')}
                                </label>
                                <div className="flex space-x-2">
                                    <input
                                        type="text"
                                        value={runtimesDialog.repository}
                                        onChange={(e) => setRuntimesDialog(prev => ({
                                            ...prev,
                                            repository: e.target.value
                                        }))}
                                        placeholder={t('resources.repositoryPlaceholder')}
                                        className="flex-1 px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                        disabled={runtimesDialog.loading}
                                    />
                                    <button
                                        onClick={() => fetchGitHubRuntimes(runtimesDialog.repository)}
                                        disabled={runtimesDialog.loading || runtimesDialog.validatingRepo}
                                        className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                    >
                                        {(runtimesDialog.loading || runtimesDialog.validatingRepo) ? (
                                            <>
                                                <div
                                                    className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                                {t('resources.validating')}
                                            </>
                                        ) : (
                                            <>
                                                <RefreshCw className="h-4 w-4 mr-2"/>
                                                {t('resources.load')}
                                            </>
                                        )}
                                    </button>
                                </div>
                                <p className="text-xs text-gray-400 mt-1">
                                    {t('resources.repositoryValidationNote')}
                                </p>
                            </div>

                            {/* Search Box */}
                            {runtimesDialog.runtimes.length > 0 && (
                                <div className="mb-4">
                                    <input
                                        type="text"
                                        value={runtimesDialog.searchQuery}
                                        onChange={(e) => setRuntimesDialog(prev => ({
                                            ...prev,
                                            searchQuery: e.target.value
                                        }))}
                                        placeholder={t('resources.searchRuntimes')}
                                        className="w-full px-3 py-2 border border-gray-600 rounded-md bg-gray-700 text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
                                    />
                                    {runtimesDialog.searchQuery && (
                                        <p className="text-xs text-gray-400 mt-1">
                                            Showing {filteredRuntimes.length} of {runtimesDialog.runtimes.length} runtimes
                                        </p>
                                    )}
                                </div>
                            )}

                            <div className="overflow-y-auto max-h-[70vh]">
                                {runtimesDialog.loading ? (
                                    <div className="text-center py-8">
                                        <div
                                            className="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-500 mx-auto mb-4"></div>
                                        <p className="text-gray-500">{t('resources.fetchingRuntimes')}</p>
                                    </div>
                                ) : runtimesDialog.error ? (
                                    <div className="text-center py-8">
                                        <p className="text-red-500 text-sm mb-4">{runtimesDialog.error}</p>
                                        <button
                                            onClick={() => fetchGitHubRuntimes()}
                                            className="inline-flex items-center px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700">
                                            <RefreshCw className="h-4 w-4 mr-2"/>
                                            Retry
                                        </button>
                                    </div>
                                ) : filteredRuntimes.length === 0 && runtimesDialog.runtimes.length > 0 ? (
                                    <div className="text-center py-8">
                                        <p className="text-gray-500">No runtimes match your search</p>
                                    </div>
                                ) : runtimesDialog.runtimes.length === 0 ? (
                                    <div className="text-center py-8">
                                        <p className="text-gray-500">No runtimes found in repository</p>
                                    </div>
                                ) : (
                                    <div className="space-y-3">
                                        {filteredRuntimes.map((runtime, index) => (
                                            <div key={runtime.name || index}
                                                 className="border border-gray-600 rounded-lg p-6 hover:bg-gray-700 transition-colors">
                                                <div className="flex items-start justify-between">
                                                    <div className="flex-1 pr-6">
                                                        {/* Header */}
                                                        <div className="flex items-start mb-3">
                                                            <Cpu className="h-6 w-6 text-purple-500 mr-3 mt-1"/>
                                                            <div className="flex-1">
                                                                <div className="flex items-center gap-3 mb-1">
                                                                    <h4 className="font-semibold text-lg text-gray-200">
                                                                        {runtime.displayName || runtime.name}
                                                                    </h4>
                                                                    {runtime.version && (
                                                                        <span
                                                                            className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200">
                                                                            v{runtime.version}
                                                                        </span>
                                                                    )}
                                                                    {runtime.language && (
                                                                        <span
                                                                            className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                                                                            {runtime.language}
                                                                        </span>
                                                                    )}
                                                                </div>
                                                                <p className="text-sm text-gray-400 font-mono">{runtime.name}</p>
                                                                {runtime.description && (
                                                                    <p className="text-sm text-gray-300 mt-2">{runtime.description}</p>
                                                                )}
                                                            </div>
                                                        </div>

                                                        {/* Details Grid */}
                                                        <div className="ml-9 space-y-3">
                                                            {/* Platforms */}
                                                            {runtime.platforms && runtime.platforms.length > 0 && (
                                                                <div>
                                                                    <p className="text-xs font-medium text-gray-400 mb-2">Supported
                                                                        Platforms:</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {runtime.platforms.map((platform) => (
                                                                            <span
                                                                                key={platform}
                                                                                className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
                                                                            >
                                                                                {platform}
                                                                            </span>
                                                                        ))}
                                                                    </div>
                                                                </div>
                                                            )}

                                                            {/* Requirements */}
                                                            {runtime.requirements && (
                                                                <div>
                                                                    <p className="text-xs font-medium text-gray-400 mb-2">System
                                                                        Requirements:</p>
                                                                    <div
                                                                        className="flex flex-wrap gap-2 text-xs text-gray-300">
                                                                        <span>RAM: {runtime.requirements.min_ram_mb}MB</span>
                                                                        <span>•</span>
                                                                        <span>Disk: {runtime.requirements.min_disk_mb}MB</span>
                                                                        {runtime.requirements.gpu_required && (
                                                                            <>
                                                                                <span>•</span>
                                                                                <span className="text-yellow-400">GPU Required</span>
                                                                            </>
                                                                        )}
                                                                    </div>
                                                                </div>
                                                            )}

                                                            {/* Executables */}
                                                            {runtime.provides?.executables && runtime.provides.executables.length > 0 && (
                                                                <div>
                                                                    <p className="text-xs font-medium text-gray-400 mb-2">Provides:</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {runtime.provides.executables.slice(0, 6).map((executable) => (
                                                                            <span
                                                                                key={executable}
                                                                                className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                                                                            >
                                                                                {executable}
                                                                            </span>
                                                                        ))}
                                                                        {runtime.provides.executables.length > 6 && (
                                                                            <span className="text-xs text-gray-400">
                                                                                +{runtime.provides.executables.length - 6} more
                                                                            </span>
                                                                        )}
                                                                    </div>
                                                                </div>
                                                            )}

                                                            {/* Tags */}
                                                            {runtime.tags && runtime.tags.length > 0 && (
                                                                <div>
                                                                    <p className="text-xs font-medium text-gray-400 mb-2">Tags:</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {runtime.tags.slice(0, 8).map((tag) => (
                                                                            <span
                                                                                key={tag}
                                                                                className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300"
                                                                            >
                                                                                #{tag}
                                                                            </span>
                                                                        ))}
                                                                        {runtime.tags.length > 8 && (
                                                                            <span className="text-xs text-gray-400">
                                                                                +{runtime.tags.length - 8} more
                                                                            </span>
                                                                        )}
                                                                    </div>
                                                                </div>
                                                            )}
                                                        </div>
                                                    </div>

                                                    {/* Action Buttons Column */}
                                                    <div className="flex flex-col items-end space-y-2 min-w-0">
                                                        {/* Install Status Indicator */}
                                                        {runtime.isInstalled ? (
                                                            <span
                                                                className="inline-flex items-center px-3 py-1 rounded-md text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 w-full justify-center">
                                                                ✓ Installed
                                                            </span>
                                                        ) : (
                                                            <span
                                                                className="inline-flex items-center px-3 py-1 rounded-md text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300 w-full justify-center">
                                                                Not Installed
                                                            </span>
                                                        )}

                                                        {/* Action Buttons */}
                                                        {runtime.html_url && (
                                                            <a
                                                                href={runtime.html_url}
                                                                target="_blank"
                                                                rel="noopener noreferrer"
                                                                className="inline-flex items-center px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-600 transition-colors w-full justify-center"
                                                            >
                                                                <ExternalLink className="h-4 w-4 mr-2"/>
                                                                View
                                                            </a>
                                                        )}

                                                        {!runtime.isInstalled ? (
                                                            <button
                                                                onClick={() => showRuntimeConfirmation('install', runtime.name)}
                                                                className="inline-flex items-center px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-sm font-medium transition-colors w-full justify-center"
                                                                title={t('resources.installRuntime')}
                                                            >
                                                                <Download className="h-4 w-4 mr-2"/>
                                                                Install
                                                            </button>
                                                        ) : (
                                                            <>
                                                                <button
                                                                    onClick={() => showRuntimeConfirmation('reinstall', runtime.name)}
                                                                    className="inline-flex items-center px-4 py-2 bg-orange-600 hover:bg-orange-700 text-white rounded-md text-sm font-medium transition-colors w-full justify-center"
                                                                    title={t('resources.reinstallRuntime')}
                                                                >
                                                                    <RefreshCw className="h-4 w-4 mr-2"/>
                                                                    Reinstall
                                                                </button>
                                                                <button
                                                                    onClick={() => showRuntimeConfirmation('remove', runtime.name)}
                                                                    className="inline-flex items-center px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-md text-sm font-medium transition-colors w-full justify-center"
                                                                    title={t('resources.removeRuntime')}
                                                                >
                                                                    <Trash2 className="h-4 w-4 mr-2"/>
                                                                    Remove
                                                                </button>
                                                            </>
                                                        )}
                                                    </div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>

                            <div className="flex justify-between items-center mt-6 pt-4 border-t border-gray-600">
                                <p className="text-xs text-gray-400">
                                    Fetched from GitHub repository
                                </p>
                                <div className="flex space-x-3">
                                    <button
                                        onClick={() => fetchGitHubRuntimes()}
                                        disabled={runtimesDialog.loading}
                                        className="inline-flex items-center px-3 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                    >
                                        <RefreshCw className="h-4 w-4 mr-2"/>
                                        Refresh
                                    </button>
                                    <button
                                        onClick={closeRuntimesDialog}
                                        className="px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white rounded-md text-sm font-medium"
                                    >
                                        Close
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Runtime Installation Progress Dialog */}
            {installProgress.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div
                        className="relative bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[80vh] overflow-hidden">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <div>
                                    <h3 className="text-lg font-medium text-gray-200">
                                        Installing Runtime: {installProgress.runtimeName}
                                    </h3>
                                    <p className="text-sm text-gray-400">Build Job: {installProgress.buildJobId}</p>
                                </div>
                                <div className="flex items-center space-x-2">
                                    {installProgress.status === 'building' && (
                                        <div
                                            className="animate-spin rounded-full h-5 w-5 border-b-2 border-blue-500"></div>
                                    )}
                                    {installProgress.status === 'completed' && (
                                        <span className="text-green-400 text-sm">✅ Complete</span>
                                    )}
                                    {installProgress.status === 'failed' && (
                                        <span className="text-red-400 text-sm">❌ Failed</span>
                                    )}
                                    <button
                                        onClick={closeInstallProgress}
                                        className="text-gray-400 hover:text-gray-300"
                                        disabled={installProgress.status === 'building'}
                                    >
                                        <X className="h-5 w-5"/>
                                    </button>
                                </div>
                            </div>

                            <div className="bg-gray-900 rounded-lg p-4 max-h-96 overflow-y-auto">
                                <div className="font-mono text-sm space-y-1">
                                    {installProgress.logs.map((log, index) => (
                                        <div key={index} className={`
                                            ${log.startsWith('ERROR:') ? 'text-red-400' :
                                            log.startsWith('✅') ? 'text-green-400' :
                                                log.startsWith('❌') ? 'text-red-400' :
                                                    log.includes('[INFO]') ? 'text-blue-400' :
                                                        'text-gray-300'}
                                        `}>
                                            {log}
                                        </div>
                                    ))}
                                    <div ref={logsEndRef}/>
                                </div>
                            </div>

                            <div className="flex justify-between items-center mt-4 pt-4 border-t border-gray-600">
                                <p className="text-xs text-gray-400">
                                    Real-time streaming from RNX build process
                                </p>
                                <button
                                    onClick={closeInstallProgress}
                                    disabled={installProgress.status === 'building'}
                                    className="px-4 py-2 bg-gray-600 hover:bg-gray-700 disabled:bg-gray-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed"
                                >
                                    {installProgress.status === 'building' ? 'Building...' : 'Close'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Runtime Action Confirmation Dialog */}
            {runtimeConfirm.show && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    {runtimeConfirm.action === 'install' && 'Install Runtime'}
                                    {runtimeConfirm.action === 'reinstall' && 'Reinstall Runtime'}
                                    {runtimeConfirm.action === 'remove' && 'Remove Runtime'}
                                </h3>
                                <button
                                    onClick={cancelRuntimeAction}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={runtimeConfirm.processing}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        {runtimeConfirm.action === 'install' && `Install runtime "${runtimeConfirm.runtimeName}" from GitHub?`}
                                        {runtimeConfirm.action === 'reinstall' && `Reinstall runtime "${runtimeConfirm.runtimeName}" with --force flag?`}
                                        {runtimeConfirm.action === 'remove' && `Remove runtime "${runtimeConfirm.runtimeName}"?`}
                                    </p>
                                    {runtimeConfirm.action === 'install' && (
                                        <p className="text-sm text-blue-400">
                                            This will download and build the runtime from ehsaniara/joblet repository.
                                        </p>
                                    )}
                                    {runtimeConfirm.action === 'reinstall' && (
                                        <p className="text-sm text-orange-400">
                                            This will rebuild the runtime, replacing the current installation.
                                        </p>
                                    )}
                                    {runtimeConfirm.action === 'remove' && (
                                        <p className="text-sm text-red-400">
                                            This action cannot be undone. The runtime will be completely removed from
                                            the system.
                                        </p>
                                    )}
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre className={`bg-gray-900 p-4 rounded-md text-sm overflow-x-auto font-mono ${
                                        runtimeConfirm.action === 'install' ? 'text-blue-400' :
                                            runtimeConfirm.action === 'reinstall' ? 'text-orange-400' :
                                                'text-red-400'
                                    }`}>
{runtimeConfirm.action === 'install'
    ? `rnx runtime install ${runtimeConfirm.runtimeName} --github-repo=ehsaniara/joblet/tree/main/runtimes`
    : runtimeConfirm.action === 'reinstall'
        ? `rnx runtime install ${runtimeConfirm.runtimeName} --force --github-repo=ehsaniara/joblet/tree/main/runtimes`
        : `rnx runtime remove ${runtimeConfirm.runtimeName}`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelRuntimeAction}
                                    disabled={runtimeConfirm.processing}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmRuntimeAction}
                                    disabled={runtimeConfirm.processing}
                                    className={`px-4 py-2 rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center
                                        ${runtimeConfirm.action === 'install' ? 'bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 text-white' :
                                        runtimeConfirm.action === 'reinstall' ? 'bg-orange-600 hover:bg-orange-700 disabled:bg-orange-800 text-white' :
                                            'bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white'}`}
                                >
                                    {runtimeConfirm.processing ? (
                                        <>
                                            <div
                                                className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Processing...
                                        </>
                                    ) : (
                                        <>
                                            {runtimeConfirm.action === 'install' && <><Download
                                                className="h-4 w-4 mr-2"/>Install</>}
                                            {runtimeConfirm.action === 'reinstall' && <><RefreshCw
                                                className="h-4 w-4 mr-2"/>Reinstall</>}
                                            {runtimeConfirm.action === 'remove' && <><Trash2
                                                className="h-4 w-4 mr-2"/>Remove</>}
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Resources;