import React, { useEffect, useState } from 'react';
import { apiService } from '../../services/apiService';
import { useMetricsStream } from '../../hooks/useMetricsStream';
import { Activity, Clock, Cpu, HardDrive, MemoryStick, RefreshCw, Wifi, WifiOff, BarChart3 } from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area } from 'recharts';

interface JobMetricsProps {
    jobId: string;
}

interface MetricPoint {
    jobId: string;
    timestamp: number;
    sampleIntervalSeconds: number;
    cpu: {
        usage?: number;
        [key: string]: any;
    };
    memory: {
        current?: number;
        limit?: number;
        [key: string]: any;
    };
    io: {
        readBytes?: number;
        writeBytes?: number;
        [key: string]: any;
    };
    process: {
        [key: string]: any;
    };
    cgroupPath?: string;
    limits: {
        [key: string]: any;
    };
    [key: string]: any;
}

export const JobMetrics: React.FC<JobMetricsProps> = ({ jobId }) => {
    const { metrics, connected, error: streamError, clearMetrics } = useMetricsStream(jobId);
    const [fallbackMetrics, setFallbackMetrics] = useState<MetricPoint[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);
    const [refreshing, setRefreshing] = useState<boolean>(false);
    const [usingFallback, setUsingFallback] = useState<boolean>(false);

    // Fallback to HTTP when WebSocket fails
    const fetchMetrics = async (showRefresh = false) => {
        try {
            if (showRefresh) {
                setRefreshing(true);
            } else {
                setLoading(true);
            }
            setError(null);

            const metricsData = await apiService.getJobMetrics(jobId);
            setFallbackMetrics(metricsData || []);
            setUsingFallback(true);
        } catch (err) {
            console.error('Failed to fetch job metrics:', err);
            setError(err instanceof Error ? err.message : 'Failed to load metrics');
        } finally {
            setLoading(false);
            setRefreshing(false);
        }
    };

    useEffect(() => {
        // Clear previous metrics when jobId changes
        clearMetrics();
        setFallbackMetrics([]);
        setUsingFallback(false);
        setError(null);
        setLoading(true);

        // For now, skip WebSocket and go directly to HTTP
        fetchMetrics();
    }, [jobId, clearMetrics]);

    useEffect(() => {
        // Handle WebSocket connection and data
        if (connected || metrics.length > 0) {
            setLoading(false);
            setUsingFallback(false);
        }

        // Set error from stream if there's one
        if (streamError) {
            setError(streamError);
            setLoading(false);
            // Try fallback after WebSocket error
            fetchMetrics();
        }
    }, [connected, metrics.length, streamError]);

    const formatBytes = (bytes: number): string => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const formatDuration = (timestamp: number): string => {
        const date = new Date(timestamp * 1000); // Convert seconds to milliseconds
        return date.toLocaleTimeString();
    };

    // Use the appropriate metrics source
    const currentMetrics = usingFallback ? fallbackMetrics : metrics;

    const getLatestMetric = (): MetricPoint | null => {
        if (currentMetrics.length === 0) return null;
        return currentMetrics[currentMetrics.length - 1];
    };

    // Prepare chart data
    const prepareChartData = () => {
        return currentMetrics.map((metric, index) => ({
            time: formatDuration(metric.timestamp),
            timeIndex: index,
            cpuUsage: metric.cpu?.usagePercent || metric.cpu?.usage || 0,
            memoryMB: metric.memory?.current ? (metric.memory.current / (1024 * 1024)) : 0,
            memoryBytes: metric.memory?.current || 0,
            diskReadMB: metric.io?.totalReadBytes ? (metric.io.totalReadBytes / (1024 * 1024)) : metric.io?.readBytes ? (metric.io.readBytes / (1024 * 1024)) : 0,
            diskWriteMB: metric.io?.totalWriteBytes ? (metric.io.totalWriteBytes / (1024 * 1024)) : metric.io?.writeBytes ? (metric.io.writeBytes / (1024 * 1024)) : 0,
        }));
    };

    const chartData = prepareChartData();

    // Calculate performance statistics
    const calculateStats = () => {
        if (currentMetrics.length === 0) {
            return {
                cpu: { average: null, peak: null },
                memory: { average: null, peak: null }
            };
        }

        const cpuValues = currentMetrics
            .map(m => m.cpu?.usagePercent || m.cpu?.usage || 0)
            .filter(v => v !== undefined);

        const memoryValues = currentMetrics
            .map(m => m.memory?.current || 0)
            .filter(v => v > 0);

        const cpuAverage = cpuValues.length > 0
            ? cpuValues.reduce((a, b) => a + b, 0) / cpuValues.length
            : null;
        const cpuPeak = cpuValues.length > 0 ? Math.max(...cpuValues) : null;

        const memoryAverage = memoryValues.length > 0
            ? memoryValues.reduce((a, b) => a + b, 0) / memoryValues.length
            : null;
        const memoryPeak = memoryValues.length > 0 ? Math.max(...memoryValues) : null;

        return {
            cpu: { average: cpuAverage, peak: cpuPeak },
            memory: { average: memoryAverage, peak: memoryPeak }
        };
    };

    const stats = calculateStats();

    if (loading) {
        return (
            <div className="flex items-center justify-center py-8">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
                <span className="ml-3 text-gray-600 dark:text-gray-400">Loading metrics...</span>
            </div>
        );
    }

    if (error) {
        return (
            <div className="space-y-4">
                <div className="flex items-center justify-between">
                    <h4 className="text-lg font-medium text-gray-900 dark:text-white">Job Metrics</h4>
                    <button
                        onClick={() => fetchMetrics(true)}
                        disabled={refreshing}
                        className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600"
                    >
                        <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
                        Refresh
                    </button>
                </div>
                <div className="bg-red-100 dark:bg-red-900 border border-red-400 text-red-700 dark:text-red-300 rounded p-4">
                    <p className="font-medium">Error loading metrics</p>
                    <p className="text-sm">{error}</p>
                    <p className="text-sm mt-2">This may be normal if the job hasn't started yet or if metrics collection is not enabled.</p>
                </div>
            </div>
        );
    }

    if (currentMetrics.length === 0) {
        return (
            <div className="space-y-4">
                <div className="flex items-center justify-between">
                    <h4 className="text-lg font-medium text-gray-900 dark:text-white">Job Metrics</h4>
                    <button
                        onClick={() => fetchMetrics(true)}
                        disabled={refreshing}
                        className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600"
                    >
                        <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
                        Refresh
                    </button>
                </div>
                <div className="bg-yellow-100 dark:bg-yellow-900 border border-yellow-400 text-yellow-700 dark:text-yellow-300 rounded p-4">
                    <p className="font-medium">No metrics available</p>
                    <p className="text-sm">Metrics will appear here once the job starts running and begins collecting performance data.</p>
                </div>
            </div>
        );
    }

    const latestMetric = getLatestMetric();

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                    <h4 className="text-lg font-medium text-gray-900 dark:text-white">Job Metrics</h4>
                    <div className="flex items-center space-x-2">
                        {connected && !usingFallback ? (
                            <>
                                <Wifi className="h-4 w-4 text-green-500" />
                                <span className="text-sm text-green-600 dark:text-green-400">Live</span>
                            </>
                        ) : usingFallback ? (
                            <>
                                <WifiOff className="h-4 w-4 text-yellow-500" />
                                <span className="text-sm text-yellow-600 dark:text-yellow-400">Static</span>
                            </>
                        ) : (
                            <div className="animate-pulse flex items-center space-x-2">
                                <div className="h-4 w-4 bg-gray-300 rounded"></div>
                                <span className="text-sm text-gray-500 dark:text-gray-400">Connecting...</span>
                            </div>
                        )}
                    </div>
                </div>
                <button
                    onClick={() => fetchMetrics(true)}
                    disabled={refreshing}
                    className="inline-flex items-center px-3 py-1 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600"
                >
                    <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
                    Refresh
                </button>
            </div>

            {/* Current Metrics */}
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                <h5 className="text-md font-medium text-gray-900 dark:text-white mb-4">Current Performance</h5>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                    {(latestMetric?.cpu?.usagePercent !== undefined || latestMetric?.cpu?.usage !== undefined) && (
                        <div className="bg-white dark:bg-gray-600 rounded-lg p-3 border">
                            <div className="flex items-center">
                                <Cpu className="h-5 w-5 text-blue-500 mr-2" />
                                <div className="flex-1">
                                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">CPU Usage</p>
                                    <p className="text-lg font-semibold text-gray-900 dark:text-white">
                                        {(latestMetric.cpu.usagePercent || latestMetric.cpu.usage || 0).toFixed(1)}%
                                    </p>
                                </div>
                            </div>
                        </div>
                    )}

                    {latestMetric?.memory?.current !== undefined && (
                        <div className="bg-white dark:bg-gray-600 rounded-lg p-3 border">
                            <div className="flex items-center">
                                <MemoryStick className="h-5 w-5 text-green-500 mr-2" />
                                <div className="flex-1">
                                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Memory Usage</p>
                                    <p className="text-lg font-semibold text-gray-900 dark:text-white">
                                        {formatBytes(latestMetric.memory.current!)}
                                    </p>
                                    {latestMetric.memory.limit && (
                                        <p className="text-xs text-gray-500 dark:text-gray-400">
                                            of {formatBytes(latestMetric.memory.limit)}
                                        </p>
                                    )}
                                </div>
                            </div>
                        </div>
                    )}

                    {(latestMetric?.io?.totalReadBytes !== undefined || latestMetric?.io?.totalWriteBytes !== undefined || latestMetric?.io?.readBytes !== undefined || latestMetric?.io?.writeBytes !== undefined) && (
                        <div className="bg-white dark:bg-gray-600 rounded-lg p-3 border">
                            <div className="flex items-center">
                                <HardDrive className="h-5 w-5 text-purple-500 mr-2" />
                                <div className="flex-1">
                                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Disk I/O</p>
                                    <p className="text-sm font-semibold text-gray-900 dark:text-white">
                                        {(latestMetric.io.totalReadBytes !== undefined || latestMetric.io.readBytes !== undefined) && (
                                            <span>R: {formatBytes(latestMetric.io.totalReadBytes || latestMetric.io.readBytes || 0)}</span>
                                        )}
                                        {(latestMetric.io.totalReadBytes !== undefined || latestMetric.io.readBytes !== undefined) && (latestMetric.io.totalWriteBytes !== undefined || latestMetric.io.writeBytes !== undefined) && <br />}
                                        {(latestMetric.io.totalWriteBytes !== undefined || latestMetric.io.writeBytes !== undefined) && (
                                            <span>W: {formatBytes(latestMetric.io.totalWriteBytes || latestMetric.io.writeBytes || 0)}</span>
                                        )}
                                    </p>
                                </div>
                            </div>
                        </div>
                    )}

                    {latestMetric?.memory?.current !== undefined && (
                        <div className="bg-white dark:bg-gray-600 rounded-lg p-3 border">
                            <div className="flex items-center">
                                <Activity className="h-5 w-5 text-orange-500 mr-2" />
                                <div className="flex-1">
                                    <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Sample Interval</p>
                                    <p className="text-sm font-semibold text-gray-900 dark:text-white">
                                        {latestMetric.sampleIntervalSeconds}s
                                    </p>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Performance Charts */}
            {chartData.length > 1 && (
                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                    <div className="flex items-center mb-4">
                        <BarChart3 className="h-5 w-5 text-blue-500 mr-2" />
                        <h5 className="text-md font-medium text-gray-900 dark:text-white">Performance Trends</h5>
                    </div>

                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        {/* CPU Usage Chart */}
                        {chartData.some(d => d.cpuUsage !== undefined) && (
                            <div className="bg-white dark:bg-gray-600 rounded-lg p-4">
                                <h6 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center">
                                    <Cpu className="h-4 w-4 text-blue-500 mr-2" />
                                    CPU Usage Over Time
                                </h6>
                                <ResponsiveContainer width="100%" height={200}>
                                    <AreaChart data={chartData}>
                                        <CartesianGrid strokeDasharray="3 3" />
                                        <XAxis
                                            dataKey="timeIndex"
                                            tick={{ fontSize: 12 }}
                                            tickFormatter={(index) => `${index + 1}`}
                                        />
                                        <YAxis
                                            tick={{ fontSize: 12 }}
                                            label={{ value: 'CPU %', angle: -90, position: 'insideLeft' }}
                                        />
                                        <Tooltip
                                            labelFormatter={(index) => `Sample ${(index as number) + 1}`}
                                            formatter={(value: any) => [`${value}%`, 'CPU Usage']}
                                        />
                                        <Area
                                            type="monotone"
                                            dataKey="cpuUsage"
                                            stroke="#3b82f6"
                                            fill="#3b82f6"
                                            fillOpacity={0.3}
                                        />
                                    </AreaChart>
                                </ResponsiveContainer>
                            </div>
                        )}

                        {/* Memory Usage Chart */}
                        {chartData.some(d => d.memoryBytes > 0) && (
                            <div className="bg-white dark:bg-gray-600 rounded-lg p-4">
                                <h6 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center">
                                    <MemoryStick className="h-4 w-4 text-green-500 mr-2" />
                                    Memory Usage Over Time
                                </h6>
                                <ResponsiveContainer width="100%" height={200}>
                                    <AreaChart data={chartData}>
                                        <CartesianGrid strokeDasharray="3 3" />
                                        <XAxis
                                            dataKey="timeIndex"
                                            tick={{ fontSize: 12 }}
                                            tickFormatter={(index) => `${index + 1}`}
                                        />
                                        <YAxis
                                            tick={{ fontSize: 12 }}
                                            label={{ value: 'MB', angle: -90, position: 'insideLeft' }}
                                        />
                                        <Tooltip
                                            labelFormatter={(index) => `Sample ${(index as number) + 1}`}
                                            formatter={(value: any) => [`${value.toFixed(2)} MB`, 'Memory Usage']}
                                        />
                                        <Area
                                            type="monotone"
                                            dataKey="memoryMB"
                                            stroke="#10b981"
                                            fill="#10b981"
                                            fillOpacity={0.3}
                                        />
                                    </AreaChart>
                                </ResponsiveContainer>
                            </div>
                        )}

                        {/* Disk I/O Chart */}
                        {chartData.some(d => d.diskReadMB !== undefined || d.diskWriteMB !== undefined) && (
                            <div className="bg-white dark:bg-gray-600 rounded-lg p-4">
                                <h6 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3 flex items-center">
                                    <HardDrive className="h-4 w-4 text-purple-500 mr-2" />
                                    Disk I/O Over Time
                                </h6>
                                <ResponsiveContainer width="100%" height={200}>
                                    <LineChart data={chartData}>
                                        <CartesianGrid strokeDasharray="3 3" />
                                        <XAxis
                                            dataKey="timeIndex"
                                            tick={{ fontSize: 12 }}
                                            tickFormatter={(index) => `${index + 1}`}
                                        />
                                        <YAxis
                                            tick={{ fontSize: 12 }}
                                            label={{ value: 'MB', angle: -90, position: 'insideLeft' }}
                                        />
                                        <Tooltip
                                            labelFormatter={(index) => `Sample ${(index as number) + 1}`}
                                            formatter={(value: any, name: string) => [
                                                `${value.toFixed(2)} MB`,
                                                name === 'diskReadMB' ? 'Read' : 'Write'
                                            ]}
                                        />
                                        <Line
                                            type="monotone"
                                            dataKey="diskReadMB"
                                            stroke="#8b5cf6"
                                            strokeWidth={2}
                                            dot={{ r: 3 }}
                                        />
                                        <Line
                                            type="monotone"
                                            dataKey="diskWriteMB"
                                            stroke="#f59e0b"
                                            strokeWidth={2}
                                            dot={{ r: 3 }}
                                        />
                                    </LineChart>
                                </ResponsiveContainer>
                            </div>
                        )}

                        {/* Live Streaming Indicator */}
                        {connected && !usingFallback && (
                            <div className="bg-white dark:bg-gray-600 rounded-lg p-4 flex items-center justify-center">
                                <div className="text-center">
                                    <div className="flex items-center justify-center mb-2">
                                        <div className="animate-pulse bg-green-500 rounded-full h-3 w-3 mr-2"></div>
                                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Live Streaming</span>
                                    </div>
                                    <p className="text-xs text-gray-500 dark:text-gray-400">
                                        Charts update automatically as new metrics arrive
                                    </p>
                                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                        {currentMetrics.length} data points collected
                                    </p>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* Statistical Summary */}
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                <h5 className="text-md font-medium text-gray-900 dark:text-white mb-4">Performance Summary</h5>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                        <h6 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">CPU Usage</h6>
                        <dl className="space-y-1">
                            <div className="flex justify-between text-sm">
                                <dt className="text-gray-500 dark:text-gray-400">Average:</dt>
                                <dd className="text-gray-900 dark:text-white">
                                    {stats.cpu.average !== null ? `${stats.cpu.average.toFixed(1)}%` : 'N/A'}
                                </dd>
                            </div>
                            <div className="flex justify-between text-sm">
                                <dt className="text-gray-500 dark:text-gray-400">Peak:</dt>
                                <dd className="text-gray-900 dark:text-white">
                                    {stats.cpu.peak !== null ? `${stats.cpu.peak.toFixed(1)}%` : 'N/A'}
                                </dd>
                            </div>
                        </dl>
                    </div>
                    <div>
                        <h6 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Memory Usage</h6>
                        <dl className="space-y-1">
                            <div className="flex justify-between text-sm">
                                <dt className="text-gray-500 dark:text-gray-400">Average:</dt>
                                <dd className="text-gray-900 dark:text-white">
                                    {stats.memory.average !== null ? formatBytes(stats.memory.average) : 'N/A'}
                                </dd>
                            </div>
                            <div className="flex justify-between text-sm">
                                <dt className="text-gray-500 dark:text-gray-400">Peak:</dt>
                                <dd className="text-gray-900 dark:text-white">
                                    {stats.memory.peak !== null ? formatBytes(stats.memory.peak) : 'N/A'}
                                </dd>
                            </div>
                        </dl>
                    </div>
                </div>
            </div>

            {/* Metrics Timeline */}
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                <h5 className="text-md font-medium text-gray-900 dark:text-white mb-4">
                    Metrics Timeline ({currentMetrics.length} data points)
                </h5>
                <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-600">
                        <thead className="bg-gray-100 dark:bg-gray-800">
                            <tr>
                                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                    <Clock className="h-4 w-4 inline mr-1" />
                                    Time
                                </th>
                                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                    <Cpu className="h-4 w-4 inline mr-1" />
                                    CPU %
                                </th>
                                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                    <MemoryStick className="h-4 w-4 inline mr-1" />
                                    Memory
                                </th>
                                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                    <HardDrive className="h-4 w-4 inline mr-1" />
                                    Disk I/O
                                </th>
                                <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                    <Activity className="h-4 w-4 inline mr-1" />
                                    Interval
                                </th>
                            </tr>
                        </thead>
                        <tbody className="bg-white dark:bg-gray-700 divide-y divide-gray-200 dark:divide-gray-600">
                            {currentMetrics.slice(-10).reverse().map((metric, index) => (
                                <tr key={index} className="hover:bg-gray-50 dark:hover:bg-gray-600">
                                    <td className="px-3 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                        {formatDuration(metric.timestamp)}
                                    </td>
                                    <td className="px-3 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                        {(metric.cpu?.usagePercent !== undefined || metric.cpu?.usage !== undefined) ?
                                            `${(metric.cpu.usagePercent || metric.cpu.usage || 0).toFixed(1)}%` : '-'}
                                    </td>
                                    <td className="px-3 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                        {metric.memory?.current !== undefined ?
                                            formatBytes(metric.memory.current) : '-'}
                                    </td>
                                    <td className="px-3 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                        {(metric.io?.totalReadBytes !== undefined || metric.io?.totalWriteBytes !== undefined || metric.io?.readBytes !== undefined || metric.io?.writeBytes !== undefined) ? (
                                            <div>
                                                {(metric.io.totalReadBytes !== undefined || metric.io.readBytes !== undefined) && (
                                                    <div>R: {formatBytes(metric.io.totalReadBytes || metric.io.readBytes || 0)}</div>
                                                )}
                                                {(metric.io.totalWriteBytes !== undefined || metric.io.writeBytes !== undefined) && (
                                                    <div>W: {formatBytes(metric.io.totalWriteBytes || metric.io.writeBytes || 0)}</div>
                                                )}
                                            </div>
                                        ) : '-'}
                                    </td>
                                    <td className="px-3 py-2 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                        {metric.sampleIntervalSeconds}s
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
                {currentMetrics.length > 10 && (
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">
                        Showing last 10 of {currentMetrics.length} metrics. Use the rnx command line tool to view all metrics.
                    </p>
                )}
            </div>

            {/* Command Reference */}
            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                <h5 className="text-md font-medium text-gray-900 dark:text-white mb-4">Command Reference</h5>
                <div className="space-y-2">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            View real-time metrics
                        </label>
                        <pre className="bg-gray-900 text-green-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job metrics {jobId}
                        </pre>
                    </div>
                </div>
            </div>
        </div>
    );
};