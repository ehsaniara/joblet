// React import not needed with modern JSX transform
import {Link} from 'react-router-dom';
import {useTranslation} from 'react-i18next';
import {useJobs} from '../hooks/useJobs';
import {useMonitorStream} from '../hooks/useMonitorStream';
import {Activity, Cpu, HardDrive, Zap} from 'lucide-react';

const Dashboard: React.FC = () => {
    const {t} = useTranslation();
    const {jobs, loading: jobsLoading, error: jobsError} = useJobs();
    const {metrics, connected, error: metricsError} = useMonitorStream();

    const runningJobs = jobs.filter(job => job.status === 'RUNNING');
    const completedJobs = jobs.filter(job => job.status === 'COMPLETED');
    const failedJobs = jobs.filter(job => job.status === 'FAILED');

    const stats = [
        {
            name: t('dashboard.stats.totalJobs'),
            value: jobs.length.toString(),
            icon: Activity,
            color: 'bg-blue-500',
        },
        {
            name: t('dashboard.stats.running'),
            value: runningJobs.length.toString(),
            icon: Zap,
            color: 'bg-yellow-500',
        },
        {
            name: t('dashboard.stats.completed'),
            value: completedJobs.length.toString(),
            icon: Activity,
            color: 'bg-green-500',
        },
        {
            name: t('dashboard.stats.failed'),
            value: failedJobs.length.toString(),
            icon: Activity,
            color: 'bg-red-500',
        },
    ];

    return (
        <div className="p-6">
            <div className="mb-8">
                <h1 className="text-3xl font-bold text-gray-900 dark:text-white">{t('dashboard.title')}</h1>
                <p className="mt-2 text-gray-600 dark:text-gray-300">{t('dashboard.subtitle')}</p>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                {stats.map((stat) => {
                    const Icon = stat.icon;
                    return (
                        <div key={stat.name} className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                            <div className="flex items-center">
                                <div className={`${stat.color} rounded-lg p-3`}>
                                    <Icon className="h-6 w-6 text-white"/>
                                </div>
                                <div className="ml-4">
                                    <p className="text-sm font-medium text-gray-600 dark:text-gray-400">{stat.name}</p>
                                    <p className="text-2xl font-semibold text-gray-900 dark:text-white">{stat.value}</p>
                                </div>
                            </div>
                        </div>
                    );
                })}
            </div>

            {/* System Health */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
                <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">{t('dashboard.systemHealth')}</h3>
                    {metricsError ? (
                        <p className="text-red-500">Error: {metricsError}</p>
                    ) : metrics ? (
                        <div className="space-y-4">
                            <div className="flex items-center">
                                <Cpu className="h-5 w-5 text-gray-400 mr-3"/>
                                <div className="flex-1">
                                    <div className="flex justify-between">
                                        <span className="text-sm text-gray-600">CPU ({metrics.cpu.cores} cores)</span>
                                        <span
                                            className="text-sm font-medium">{metrics.cpu.usagePercent.toFixed(1)}%</span>
                                    </div>
                                    <div className="w-full bg-gray-200 rounded-full h-2 mt-1">
                                        <div
                                            className="bg-blue-600 h-2 rounded-full"
                                            style={{width: `${metrics.cpu.usagePercent}%`}}
                                        ></div>
                                    </div>
                                </div>
                            </div>

                            <div className="flex items-center">
                                <HardDrive className="h-5 w-5 text-gray-400 mr-3"/>
                                <div className="flex-1">
                                    <div className="flex justify-between">
                                        <span
                                            className="text-sm text-gray-600">Memory ({(metrics.memory.usedBytes / (1024 * 1024 * 1024)).toFixed(1)}GB / {(metrics.memory.totalBytes / (1024 * 1024 * 1024)).toFixed(1)}GB)</span>
                                        <span
                                            className="text-sm font-medium">{metrics.memory.usagePercent.toFixed(1)}%</span>
                                    </div>
                                    <div className="w-full bg-gray-200 rounded-full h-2 mt-1">
                                        <div
                                            className="bg-green-600 h-2 rounded-full"
                                            style={{width: `${metrics.memory.usagePercent}%`}}
                                        ></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    ) : (
                        <p className="text-gray-500 dark:text-gray-400">
                            {connected ? t('dashboard.waitingForMetrics') : t('dashboard.connectingToMonitoring')}
                        </p>
                    )}
                </div>

                <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">{t('dashboard.recentJobs')}</h3>
                    {jobsLoading ? (
                        <p className="text-white-500">{t('dashboard.loadingJobs')}</p>
                    ) : jobsError ? (
                        <p className="text-red-500">{t('common.error')}: {jobsError}</p>
                    ) : jobs.length > 0 ? (
                        <div className="space-y-3">
                            {jobs.slice(0, 5).map((job) => (
                                <div key={job.id}
                                     className="flex items-center justify-between py-2 border-b border-gray-100 last:border-0">
                                    <div>
                                        <p className="text-sm font-medium text-white-900">{job.id.slice(0, 8)}</p>
                                        <p className="text-sm text-gray-500">{job.command}</p>
                                    </div>
                                    <span
                                        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                            job.status === 'RUNNING' ? 'bg-yellow-100 text-yellow-800' :
                                                job.status === 'COMPLETED' ? 'bg-green-100 text-green-800' :
                                                    job.status === 'FAILED' ? 'bg-red-100 text-red-800' :
                                                        'bg-gray-100 text-gray-800'
                                        }`}>
                    {job.status}
                  </span>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <p className="text-gray-500">{t('jobs.noJobs')}</p>
                    )}
                </div>
            </div>

            {/* Quick Actions */}
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold text-white mb-4">{t('dashboard.quickActions')}</h3>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <Link to="/jobs"
                          className="flex items-center justify-center px-4 py-3 border border-gray-300 rounded-lg hover:bg-gray-500 transition-colors">
                        <Zap className="h-5 w-5 mr-2 text-blue-600"/>
                        {t('dashboard.runNewJob')}
                    </Link>
                    <Link to="/jobs"
                          className="flex items-center justify-center px-4 py-3 border border-gray-300 rounded-lg hover:bg-gray-500 transition-colors">
                        <Activity className="h-5 w-5 mr-2 text-green-600"/>
                        {t('dashboard.viewAllJobs')}
                    </Link>
                    <Link to="/resources"
                          className="flex items-center justify-center px-4 py-3 border border-gray-300 rounded-lg hover:bg-gray-500 transition-colors">
                        <HardDrive className="h-5 w-5 mr-2 text-purple-600"/>
                        {t('dashboard.manageResources')}
                    </Link>
                </div>
            </div>
        </div>
    );
};

export default Dashboard;