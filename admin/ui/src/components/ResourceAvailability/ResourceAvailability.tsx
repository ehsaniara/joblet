import React, {useEffect, useState} from 'react';
import {DetailedSystemInfo} from '../../types/monitor';
import {apiService} from '../../services/apiService';
import {Cpu, HardDrive, MemoryStick, RefreshCw, Server, Zap} from 'lucide-react';

interface ResourceAvailabilityProps {
    className?: string;
    showTitle?: boolean;
}

const formatBytes = (bytes: number): string => {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatPercentage = (value: number): string => {
    return `${Math.round(value)}%`;
};

export const ResourceAvailability: React.FC<ResourceAvailabilityProps> = ({
                                                                              className = '',
                                                                              showTitle = true
                                                                          }) => {
    const [systemInfo, setSystemInfo] = useState<DetailedSystemInfo | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string>('');
    const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

    const fetchSystemInfo = async () => {
        try {
            setLoading(true);
            setError('');
            const info = await apiService.getDetailedSystemInfo();
            setSystemInfo(info);
            setLastUpdated(new Date());
        } catch (err) {
            setError(`Failed to load system information: ${err instanceof Error ? err.message : 'Unknown error'}`);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchSystemInfo();
    }, []);

    if (loading) {
        return (
            <div className={`bg-gray-50 dark:bg-gray-700 rounded-lg p-4 ${className}`}>
                <div className="animate-pulse">
                    {showTitle && <div className="h-5 bg-gray-300 dark:bg-gray-600 rounded mb-3 w-32"></div>}
                    <div className="space-y-3">
                        <div className="h-12 bg-gray-300 dark:bg-gray-600 rounded"></div>
                        <div className="h-12 bg-gray-300 dark:bg-gray-600 rounded"></div>
                        <div className="h-12 bg-gray-300 dark:bg-gray-600 rounded"></div>
                    </div>
                </div>
            </div>
        );
    }

    if (error) {
        return (
            <div
                className={`bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 ${className}`}>
                <div className="flex items-center justify-between">
                    <div className="flex items-center">
                        <div className="text-red-600 dark:text-red-400 text-sm">
                            {error}
                        </div>
                    </div>
                    <button
                        onClick={fetchSystemInfo}
                        className="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300"
                    >
                        <RefreshCw className="w-4 h-4"/>
                    </button>
                </div>
            </div>
        );
    }

    if (!systemInfo) return null;

    const cpuUsage = systemInfo.cpuInfo?.usage || 0;
    const memoryUsage = systemInfo.memoryInfo?.percent || 0;
    const availableMemoryGB = systemInfo.memoryInfo?.available ?
        Math.round(systemInfo.memoryInfo.available / (1024 * 1024 * 1024)) : 0;
    const totalMemoryGB = systemInfo.memoryInfo?.total ?
        Math.round(systemInfo.memoryInfo.total / (1024 * 1024 * 1024)) : 0;

    return (
        <div className={`bg-gray-50 dark:bg-gray-700 rounded-lg p-4 ${className}`}>
            {showTitle && (
                <div className="flex items-center justify-between mb-4">
                    <h4 className="text-lg font-medium text-gray-900 dark:text-white">Available Resources</h4>
                    <div className="flex items-center space-x-2">
                        {lastUpdated && (
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                                Updated {lastUpdated.toLocaleTimeString()}
                            </span>
                        )}
                        <button
                            onClick={fetchSystemInfo}
                            className="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
                        >
                            <RefreshCw className="w-4 h-4"/>
                        </button>
                    </div>
                </div>
            )}

            {/* Node Information */}
            {(systemInfo.hostInfo?.nodeId || systemInfo.hostInfo?.serverIPs?.length || systemInfo.hostInfo?.macAddresses?.length) && (
                <div
                    className="mb-4 p-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-600">
                    <div className="flex items-center space-x-2 mb-3">
                        <Server className="w-4 h-4 text-indigo-500"/>
                        <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300">Node Information</h5>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 text-xs">
                        {systemInfo.hostInfo.nodeId && (
                            <div>
                                <dt className="font-medium text-gray-500 dark:text-gray-400">Node ID</dt>
                                <dd className="mt-1 font-mono text-gray-900 dark:text-white">{systemInfo.hostInfo.nodeId}</dd>
                            </div>
                        )}
                        {systemInfo.hostInfo.serverIPs && systemInfo.hostInfo.serverIPs.length > 0 && (
                            <div>
                                <dt className="font-medium text-gray-500 dark:text-gray-400">IP Addresses</dt>
                                <dd className="mt-1 space-y-1">
                                    {systemInfo.hostInfo.serverIPs.map((ip, index) => (
                                        <div key={index} className="font-mono text-gray-900 dark:text-white text-xs">
                                            {ip}
                                        </div>
                                    ))}
                                </dd>
                            </div>
                        )}
                        {systemInfo.hostInfo.macAddresses && systemInfo.hostInfo.macAddresses.length > 0 && (
                            <div>
                                <dt className="font-medium text-gray-500 dark:text-gray-400">MAC Addresses</dt>
                                <dd className="mt-1 space-y-1">
                                    {systemInfo.hostInfo.macAddresses.map((mac, index) => (
                                        <div key={index} className="font-mono text-gray-900 dark:text-white text-xs">
                                            {mac}
                                        </div>
                                    ))}
                                </dd>
                            </div>
                        )}
                        {systemInfo.hostInfo.hostname && (
                            <div>
                                <dt className="font-medium text-gray-500 dark:text-gray-400">Hostname</dt>
                                <dd className="mt-1 font-mono text-gray-900 dark:text-white">{systemInfo.hostInfo.hostname}</dd>
                            </div>
                        )}
                    </div>
                </div>
            )}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {/* CPU */}
                <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
                    <div className="flex items-center space-x-2 mb-2">
                        <Cpu className="w-4 h-4 text-blue-500"/>
                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">CPU</span>
                    </div>
                    <div className="space-y-1">
                        <div className="text-lg font-semibold text-gray-900 dark:text-white">
                            {systemInfo.cpuInfo?.cores || 0} cores
                        </div>
                        <div className="flex items-center space-x-2">
                            <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                <div
                                    className={`h-2 rounded-full transition-all ${
                                        cpuUsage > 80 ? 'bg-red-500' :
                                            cpuUsage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                                    }`}
                                    style={{width: `${Math.min(cpuUsage, 100)}%`}}
                                ></div>
                            </div>
                            <span className="text-xs text-gray-600 dark:text-gray-400">
                                {formatPercentage(cpuUsage)}
                            </span>
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">
                            Available: {formatPercentage(100 - cpuUsage)}
                        </div>
                    </div>
                </div>

                {/* Memory */}
                <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
                    <div className="flex items-center space-x-2 mb-2">
                        <MemoryStick className="w-4 h-4 text-green-500"/>
                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Memory</span>
                    </div>
                    <div className="space-y-1">
                        <div className="text-lg font-semibold text-gray-900 dark:text-white">
                            {availableMemoryGB}GB free
                        </div>
                        <div className="flex items-center space-x-2">
                            <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                <div
                                    className={`h-2 rounded-full transition-all ${
                                        memoryUsage > 80 ? 'bg-red-500' :
                                            memoryUsage > 60 ? 'bg-yellow-500' : 'bg-green-500'
                                    }`}
                                    style={{width: `${Math.min(memoryUsage, 100)}%`}}
                                ></div>
                            </div>
                            <span className="text-xs text-gray-600 dark:text-gray-400">
                                {formatPercentage(memoryUsage)}
                            </span>
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">
                            {availableMemoryGB}GB / {totalMemoryGB}GB
                        </div>
                    </div>
                </div>

                {/* GPU */}
                {systemInfo.gpuInfo?.totalGpus && systemInfo.gpuInfo.totalGpus > 0 ? (
                    <div
                        className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
                        <div className="flex items-center space-x-2 mb-2">
                            <Zap className="w-4 h-4 text-purple-500"/>
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">GPU</span>
                        </div>
                        <div className="space-y-1">
                            <div className="text-lg font-semibold text-gray-900 dark:text-white">
                                {systemInfo.gpuInfo.totalGpus} GPU{systemInfo.gpuInfo.totalGpus > 1 ? 's' : ''}
                            </div>
                            {systemInfo.gpuInfo.gpus && systemInfo.gpuInfo.gpus.length > 0 && (
                                <>
                                    <div className="flex items-center space-x-2">
                                        <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                            <div
                                                className={`h-2 rounded-full transition-all ${
                                                    systemInfo.gpuInfo.gpus[0].utilizationGpu > 80 ? 'bg-red-500' :
                                                        systemInfo.gpuInfo.gpus[0].utilizationGpu > 60 ? 'bg-yellow-500' : 'bg-green-500'
                                                }`}
                                                style={{width: `${Math.min(systemInfo.gpuInfo.gpus[0].utilizationGpu, 100)}%`}}
                                            ></div>
                                        </div>
                                        <span className="text-xs text-gray-600 dark:text-gray-400">
                                            {formatPercentage(systemInfo.gpuInfo.gpus[0].utilizationGpu)}
                                        </span>
                                    </div>
                                    <div className="text-xs text-gray-500 dark:text-gray-400">
                                        {formatBytes(systemInfo.gpuInfo.gpus[0].memoryFree)} free
                                    </div>
                                </>
                            )}
                        </div>
                    </div>
                ) : (
                    <div
                        className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
                        <div className="flex items-center space-x-2 mb-2">
                            <Zap className="w-4 h-4 text-gray-400"/>
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">GPU</span>
                        </div>
                        <div className="space-y-1">
                            <div className="text-lg font-semibold text-gray-400 dark:text-gray-500">
                                No GPUs
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400">
                                No GPU resources available
                            </div>
                        </div>
                    </div>
                )}

                {/* Storage */}
                <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
                    <div className="flex items-center space-x-2 mb-2">
                        <HardDrive className="w-4 h-4 text-orange-500"/>
                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Storage</span>
                    </div>
                    <div className="space-y-1">
                        {systemInfo.disksInfo?.disks && systemInfo.disksInfo.disks.length > 0 ? (
                            <>
                                <div className="text-lg font-semibold text-gray-900 dark:text-white">
                                    {formatBytes(systemInfo.disksInfo.disks[0].available)} free
                                </div>
                                <div className="flex items-center space-x-2">
                                    <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                        <div
                                            className={`h-2 rounded-full transition-all ${
                                                systemInfo.disksInfo.disks[0].percent > 80 ? 'bg-red-500' :
                                                    systemInfo.disksInfo.disks[0].percent > 60 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{width: `${Math.min(systemInfo.disksInfo.disks[0].percent, 100)}%`}}
                                        ></div>
                                    </div>
                                    <span className="text-xs text-gray-600 dark:text-gray-400">
                                        {formatPercentage(systemInfo.disksInfo.disks[0].percent)}
                                    </span>
                                </div>
                                <div className="text-xs text-gray-500 dark:text-gray-400">
                                    {formatBytes(systemInfo.disksInfo.disks[0].available)} / {formatBytes(systemInfo.disksInfo.disks[0].size)}
                                </div>
                            </>
                        ) : (
                            <div className="text-sm text-gray-500 dark:text-gray-400">
                                Storage info unavailable
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Additional GPU Details */}
            {systemInfo.gpuInfo?.gpus && systemInfo.gpuInfo.gpus.length > 1 && (
                <div className="mt-4">
                    <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">GPU Details</h5>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                        {systemInfo.gpuInfo.gpus.map((gpu) => (
                            <div key={gpu.id}
                                 className="bg-white dark:bg-gray-800 rounded p-2 border border-gray-200 dark:border-gray-600">
                                <div className="text-sm font-medium text-gray-900 dark:text-white">
                                    GPU {gpu.id}: {gpu.name}
                                </div>
                                <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                    {formatBytes(gpu.memoryFree)} free / {formatBytes(gpu.memoryTotal)} total
                                </div>
                                <div className="flex items-center space-x-2 mt-1">
                                    <div className="flex-1 bg-gray-200 dark:bg-gray-600 rounded-full h-1">
                                        <div
                                            className={`h-1 rounded-full transition-all ${
                                                gpu.utilizationGpu > 80 ? 'bg-red-500' :
                                                    gpu.utilizationGpu > 60 ? 'bg-yellow-500' : 'bg-green-500'
                                            }`}
                                            style={{width: `${Math.min(gpu.utilizationGpu, 100)}%`}}
                                        ></div>
                                    </div>
                                    <span className="text-xs text-gray-600 dark:text-gray-400">
                                        {formatPercentage(gpu.utilizationGpu)}
                                    </span>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Resource Usage Recommendations */}
            <div
                className="mt-4 p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
                <h5 className="text-sm font-medium text-blue-900 dark:text-blue-300 mb-2">Resource Recommendations</h5>
                <div className="text-xs text-blue-800 dark:text-blue-400 space-y-1">
                    <div>• Consider CPU usage when setting CPU limits (current: {formatPercentage(cpuUsage)})</div>
                    <div>• Available memory for jobs: ~{availableMemoryGB}GB</div>
                    {systemInfo.gpuInfo?.totalGpus && systemInfo.gpuInfo.totalGpus > 0 ? (
                        <div>• {systemInfo.gpuInfo.totalGpus} GPU{systemInfo.gpuInfo.totalGpus > 1 ? 's' : ''} available
                            for GPU-accelerated workloads</div>
                    ) : (
                        <div>• No GPUs available - CPU-only workloads</div>
                    )}
                </div>
            </div>
        </div>
    );
};