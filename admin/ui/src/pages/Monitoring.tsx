import React from 'react';
import {useMonitor} from '../hooks/useMonitor';
import {Activity, Cpu, HardDrive, Network} from 'lucide-react';

const Monitoring: React.FC = () => {
    const {metrics, loading, error, isRealtime, toggleRealtime} = useMonitor();

    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-900">System Monitoring</h1>
                        <p className="mt-2 text-gray-600">Real-time system metrics and performance</p>
                    </div>
                    <button
                        onClick={toggleRealtime}
                        className={`inline-flex items-center px-4 py-2 rounded-md text-sm font-medium ${
                            isRealtime
                                ? 'bg-green-600 text-white hover:bg-green-700'
                                : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
                        }`}
                    >
                        <Activity className="h-4 w-4 mr-2"/>
                        {isRealtime ? 'Real-time ON' : 'Real-time OFF'}
                    </button>
                </div>
            </div>

            {loading ? (
                <div className="bg-white rounded-lg shadow p-6">
                    <p className="text-gray-500">Loading metrics...</p>
                </div>
            ) : error ? (
                <div className="bg-white rounded-lg shadow p-6">
                    <p className="text-red-500">Error: {error}</p>
                </div>
            ) : metrics ? (
                <div className="space-y-6">
                    {/* CPU Usage */}
                    <div className="bg-auto rounded-lg shadow p-6">
                        <div className="flex items-center mb-4">
                            <Cpu className="h-6 w-6 text-blue-600 mr-3"/>
                            <h3 className="text-lg font-semibold text-gray-900">
                                CPU Usage ({metrics.cpu.cores} cores)
                            </h3>
                        </div>
                        <div className="space-y-4">
                            <div>
                                <div className="flex justify-between mb-2">
                                    <span className="text-sm text-gray-600">Overall Usage</span>
                                    <span className="text-sm font-medium">{metrics.cpu.usage}%</span>
                                </div>
                                <div className="w-full bg-gray-200 rounded-full h-3">
                                    <div
                                        className="bg-blue-600 h-3 rounded-full transition-all duration-300"
                                        style={{width: `${metrics.cpu.usage}%`}}
                                    ></div>
                                </div>
                            </div>
                            <div className="grid grid-cols-3 gap-4 text-sm">
                                <div>
                                    <span className="text-gray-600">Load Average:</span>
                                    <div className="font-medium">
                                        {metrics.cpu.loadAverage?.join(', ') || 'N/A'}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Memory Usage */}
                    <div className="bg-white rounded-lg shadow p-6">
                        <div className="flex items-center mb-4">
                            <HardDrive className="h-6 w-6 text-green-600 mr-3"/>
                            <h3 className="text-lg font-semibold text-gray-900">Memory Usage</h3>
                        </div>
                        <div className="space-y-4">
                            <div>
                                <div className="flex justify-between mb-2">
                                    <span className="text-sm text-gray-600">Memory Usage</span>
                                    <span className="text-sm font-medium">
                    {((metrics.memory.used / 1024 / 1024 / 1024) || 0).toFixed(1)} GB /
                                        {((metrics.memory.total / 1024 / 1024 / 1024) || 0).toFixed(1)} GB
                    ({metrics.memory.percent}%)
                  </span>
                                </div>
                                <div className="w-full bg-gray-200 rounded-full h-3">
                                    <div
                                        className="bg-green-600 h-3 rounded-full transition-all duration-300"
                                        style={{width: `${metrics.memory.percent}%`}}
                                    ></div>
                                </div>
                            </div>
                            <div className="grid grid-cols-3 gap-4 text-sm">
                                <div>
                                    <span className="text-gray-600">Available:</span>
                                    <div className="font-medium">
                                        {((metrics.memory.available / 1024 / 1024 / 1024) || 0).toFixed(1)} GB
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Disk I/O */}
                    <div className="bg-white rounded-lg shadow p-6">
                        <div className="flex items-center mb-4">
                            <Network className="h-6 w-6 text-purple-600 mr-3"/>
                            <h3 className="text-lg font-semibold text-gray-900">Disk I/O</h3>
                        </div>
                        <div className="grid grid-cols-3 gap-4">
                            <div className="text-center">
                                <div className="text-2xl font-bold text-purple-600">
                                    {((metrics.disk.readBps / 1024 / 1024) || 0).toFixed(1)}
                                </div>
                                <div className="text-sm text-gray-600">MB/s Read</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-purple-600">
                                    {((metrics.disk.writeBps / 1024 / 1024) || 0).toFixed(1)}
                                </div>
                                <div className="text-sm text-gray-600">MB/s Write</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-purple-600">
                                    {metrics.disk.iops || 0}
                                </div>
                                <div className="text-sm text-gray-600">IOPS</div>
                            </div>
                        </div>
                    </div>

                    {/* Job Statistics */}
                    <div className="bg-white rounded-lg shadow p-6">
                        <div className="flex items-center mb-4">
                            <Activity className="h-6 w-6 text-orange-600 mr-3"/>
                            <h3 className="text-lg font-semibold text-gray-900">Job Statistics</h3>
                        </div>
                        <div className="grid grid-cols-4 gap-4">
                            <div className="text-center">
                                <div className="text-2xl font-bold text-blue-600">{metrics.jobs.total}</div>
                                <div className="text-sm text-gray-600">Total</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-yellow-600">{metrics.jobs.running}</div>
                                <div className="text-sm text-gray-600">Running</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-green-600">{metrics.jobs.completed}</div>
                                <div className="text-sm text-gray-600">Completed</div>
                            </div>
                            <div className="text-center">
                                <div className="text-2xl font-bold text-red-600">{metrics.jobs.failed}</div>
                                <div className="text-sm text-gray-600">Failed</div>
                            </div>
                        </div>
                    </div>
                </div>
            ) : (
                <div className="bg-white rounded-lg shadow p-6">
                    <p className="text-gray-500">No metrics available</p>
                </div>
            )}
        </div>
    );
};

export default Monitoring;