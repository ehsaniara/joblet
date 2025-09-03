// React import not needed with modern JSX transform
import {useTranslation} from 'react-i18next';
import {useMonitorStream} from '../hooks/useMonitorStream';
import {useSystemInfo} from '../hooks/useSystemInfo';
import {useVolumes} from '../hooks/useVolumes';
import HostInfoCard from '../components/Monitoring/HostInfoCard';
import CPUDetailsCard from '../components/Monitoring/CPUDetailsCard';
import MemoryDetailsCard from '../components/Monitoring/MemoryDetailsCard';
import DisksCard from '../components/Monitoring/DisksCard';
import VolumesCard from '../components/Monitoring/VolumesCard';
import NetworkCard from '../components/Monitoring/NetworkCard';
import ProcessesCard from '../components/Monitoring/ProcessesCard';

const Monitoring: React.FC = () => {
    const {t} = useTranslation();
    const {metrics, connected, error: metricsError} = useMonitorStream();
    const {systemInfo, loading: systemLoading, error: systemError} = useSystemInfo();
    const {volumes, loading: volumesLoading, error: volumesError} = useVolumes();

    const loading = systemLoading || volumesLoading;
    const error = metricsError || systemError || volumesError;

    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">{t('monitoring.title')}</h1>
                        <p className="mt-2 text-gray-300">{t('monitoring.subtitle')}</p>
                        <div className="mt-2 flex items-center text-sm">
                            <div
                                className={`w-2 h-2 rounded-full mr-2 ${connected ? 'bg-green-500 animate-pulse' : 'bg-yellow-500'}`}></div>
                            <span className="text-gray-400">
                                {connected ? t('monitoring.liveUpdates') : t('monitoring.connecting')}
                            </span>
                        </div>
                    </div>
                </div>
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow p-6">
                    <p className="text-gray-300">{t('monitoring.loadingSystemInfo')}</p>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow p-6">
                    <p className="text-red-400">{t('common.error')}: {error}</p>
                </div>
            ) : (
                <div className="space-y-6">
                    {/* Host Information */}
                    {systemInfo?.hostInfo && (
                        <HostInfoCard hostInfo={systemInfo.hostInfo}/>
                    )}

                    {/* CPU Details */}
                    {systemInfo?.cpuInfo && (
                        <CPUDetailsCard
                            cpuInfo={{
                                ...systemInfo.cpuInfo,
                                ...(metrics?.cpu && {
                                    usage: metrics.cpu.usagePercent,
                                    loadAverage: metrics.cpu.loadAverage,
                                    perCoreUsage: metrics.cpu.perCoreUsage
                                })
                            }}
                        />
                    )}

                    {/* Memory Details */}
                    {systemInfo?.memoryInfo && (
                        <MemoryDetailsCard
                            memoryInfo={{
                                ...systemInfo.memoryInfo,
                                ...(metrics?.memory && {
                                    used: metrics.memory.usedBytes,
                                    available: metrics.memory.availableBytes,
                                    total: metrics.memory.totalBytes,
                                    percent: metrics.memory.usagePercent,
                                    buffers: metrics.memory.bufferedBytes,
                                    cached: metrics.memory.cachedBytes
                                })
                            }}
                        />
                    )}

                    {/* Disk Information */}
                    {systemInfo?.disksInfo && (
                        <DisksCard disksInfo={systemInfo.disksInfo}/>
                    )}

                    {/* Volume Information */}
                    {volumes && volumes.length > 0 && (
                        <VolumesCard volumes={volumes}/>
                    )}

                    {/* Network Interfaces */}
                    {systemInfo?.networkInfo && (
                        <NetworkCard networkInfo={systemInfo.networkInfo}/>
                    )}

                    {/* Running Processes */}
                    {systemInfo?.processesInfo && (
                        <ProcessesCard processesInfo={systemInfo.processesInfo}/>
                    )}

                </div>
            )}
        </div>
    );
};

export default Monitoring;