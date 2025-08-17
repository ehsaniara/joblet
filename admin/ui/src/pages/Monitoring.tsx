import React from 'react';
import {useMonitor} from '../hooks/useMonitor';
import {useSystemInfo} from '../hooks/useSystemInfo';
import {Activity, RefreshCw} from 'lucide-react';
import HostInfoCard from '../components/Monitoring/HostInfoCard';
import CPUDetailsCard from '../components/Monitoring/CPUDetailsCard';
import MemoryDetailsCard from '../components/Monitoring/MemoryDetailsCard';
import DisksCard from '../components/Monitoring/DisksCard';
import NetworkCard from '../components/Monitoring/NetworkCard';
import ProcessesCard from '../components/Monitoring/ProcessesCard';

const Monitoring: React.FC = () => {
    const {metrics, loading: metricsLoading, error: metricsError, isRealtime, toggleRealtime} = useMonitor();
    const {systemInfo, loading: systemLoading, error: systemError, refetch} = useSystemInfo();
    
    const loading = metricsLoading || systemLoading;
    const error = metricsError || systemError;

    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">System Monitoring</h1>
                        <p className="mt-2 text-gray-300">Real-time system metrics and performance</p>
                    </div>
                    <div className="flex space-x-3">
                        <button
                            onClick={refetch}
                            className="inline-flex items-center px-4 py-2 rounded-md text-sm font-medium bg-blue-600 text-white hover:bg-blue-700"
                        >
                            <RefreshCw className="h-4 w-4 mr-2"/>
                            Refresh
                        </button>
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
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow p-6">
                    <p className="text-gray-300">Loading system information...</p>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow p-6">
                    <p className="text-red-400">Error: {error}</p>
                </div>
            ) : (
                <div className="space-y-6">
                    {/* Host Information */}
                    {systemInfo?.hostInfo && (
                        <HostInfoCard hostInfo={systemInfo.hostInfo} />
                    )}

                    {/* CPU Details */}
                    {systemInfo?.cpuInfo && (
                        <CPUDetailsCard cpuInfo={systemInfo.cpuInfo} />
                    )}

                    {/* Memory Details */}
                    {systemInfo?.memoryInfo && (
                        <MemoryDetailsCard memoryInfo={systemInfo.memoryInfo} />
                    )}

                    {/* Disk Information */}
                    {systemInfo?.disksInfo && (
                        <DisksCard disksInfo={systemInfo.disksInfo} />
                    )}

                    {/* Network Interfaces */}
                    {systemInfo?.networkInfo && (
                        <NetworkCard networkInfo={systemInfo.networkInfo} />
                    )}

                    {/* Running Processes */}
                    {systemInfo?.processesInfo && (
                        <ProcessesCard processesInfo={systemInfo.processesInfo} />
                    )}

                    {/* Job Statistics */}
                    {metrics && (
                        <div className="bg-gray-800 rounded-lg shadow p-6">
                            <div className="flex items-center mb-4">
                                <Activity className="h-6 w-6 text-orange-600 mr-3"/>
                                <h3 className="text-lg font-semibold text-white">Job Statistics</h3>
                            </div>
                            <div className="grid grid-cols-4 gap-4">
                                <div className="text-center bg-gray-900 rounded-lg shadow p-6">
                                    <div className="text-2xl font-bold text-blue-600">{metrics.jobs.total}</div>
                                    <div className="text-sm text-gray-400">Total</div>
                                </div>
                                <div className="text-center bg-gray-900 rounded-lg shadow p-6">
                                    <div className="text-2xl font-bold text-yellow-600">{metrics.jobs.running}</div>
                                    <div className="text-sm text-gray-400">Running</div>
                                </div>
                                <div className="text-center bg-gray-900 rounded-lg shadow p-6">
                                    <div className="text-2xl font-bold text-green-600">{metrics.jobs.completed}</div>
                                    <div className="text-sm text-gray-400">Completed</div>
                                </div>
                                <div className="text-center bg-gray-900 rounded-lg shadow p-6">
                                    <div className="text-2xl font-bold text-red-600">{metrics.jobs.failed}</div>
                                    <div className="text-sm text-gray-400">Failed</div>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};

export default Monitoring;