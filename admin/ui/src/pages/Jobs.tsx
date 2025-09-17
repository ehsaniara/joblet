import {useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useJobs} from '../hooks/useJobs';
import {useLogStream} from '../hooks/useLogStream';
import {apiService} from '../services/apiService';
import {Job} from '../types/job';
import {ChevronLeft, ChevronRight, FileText, Play, Plus, Square, Trash2, X} from 'lucide-react';
import {SimpleJobBuilder} from '../components/JobBuilder/SimpleJobBuilder';

const Jobs: React.FC = () => {
    const {t} = useTranslation();
    const {
        loading,
        error,
        currentPage,
        pageSize,
        totalJobs,
        totalPages,
        paginatedJobs,
        setCurrentPage,
        setPageSize,
        stopJob,
        deleteJob,
        refreshJobs,
        deleteAllJobs
    } = useJobs();
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [activeTab, setActiveTab] = useState<'logs' | 'details'>('logs');
    const [selectedJob, setSelectedJob] = useState<Job | null>(null);
    const [jobLoading, setJobLoading] = useState<boolean>(false);
    const [stoppingJobId, setStoppingJobId] = useState<string | null>(null);
    const [deletingJobId, setDeletingJobId] = useState<string | null>(null);
    const [autoScroll, setAutoScroll] = useState<boolean>(true);
    const [showCreateJob, setShowCreateJob] = useState<boolean>(false);
    const [stopJobConfirm, setStopJobConfirm] = useState<{
        show: boolean;
        jobId: string;
        stopping: boolean;
    }>({
        show: false,
        jobId: '',
        stopping: false
    });
    const [deleteAllConfirm, setDeleteAllConfirm] = useState<{
        show: boolean;
        deleting: boolean;
    }>({
        show: false,
        deleting: false
    });
    const [deleteJobConfirm, setDeleteJobConfirm] = useState<{
        show: boolean;
        jobId: string;
        deleting: boolean;
    }>({
        show: false,
        jobId: '',
        deleting: false
    });
    const {logs, connected, error: logError, clearLogs} = useLogStream(selectedJobId);
    const logContainerRef = useRef<HTMLDivElement>(null);

    const handleJobCreated = () => {
        setShowCreateJob(false);
        // Immediately refresh the jobs list to show the new job
        refreshJobs();
    };

    const handleCloseCreateJob = () => {
        setShowCreateJob(false);
    };

    const getStatusColor = (status: string) => {
        switch (status) {
            case 'RUNNING':
                return 'bg-yellow-100 text-yellow-800';
            case 'COMPLETED':
                return 'bg-green-100 text-green-800';
            case 'FAILED':
                return 'bg-red-100 text-red-800';
            case 'STOPPED':
                return 'bg-gray-100 text-gray-800';
            case 'QUEUED':
                return 'bg-blue-100 text-blue-800';
            default:
                return 'bg-gray-100 text-gray-800';
        }
    };

    const formatDuration = (duration: number) => {
        if (!duration) return '-';
        const seconds = Math.floor(duration / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds % 60}s`;
        } else {
            return `${seconds}s`;
        }
    };

    const shortenUuid = (uuid: string) => {
        if (!uuid) return '-';
        // If it looks like a UUID (contains hyphens or is long), show first 8 characters
        if (uuid.includes('-') || uuid.length > 12) {
            return uuid.substring(0, 8);
        }
        // Otherwise return as-is (might already be short)
        return uuid;
    };

    const handleViewJob = async (jobId: string) => {
        setSelectedJobId(jobId);
        setActiveTab('logs');
        setJobLoading(true);

        try {
            // Try to get comprehensive status first
            try {
                const statusData = await apiService.getJobStatus(jobId);
                // Map the comprehensive status data to Job interface
                const enhancedJob: Job = {
                    id: statusData.uuid || statusData.id || jobId,
                    name: statusData.name,
                    command: statusData.command || '',
                    args: statusData.args || [],
                    status: statusData.status || 'UNKNOWN',
                    createdTime: statusData.created_time || statusData.createdTime,
                    startTime: statusData.start_time || statusData.startTime || '',
                    endTime: statusData.end_time || statusData.endTime,
                    scheduledTime: statusData.scheduled_time || statusData.scheduledTime,
                    duration: statusData.duration || 0,
                    exitCode: statusData.exit_code ?? statusData.exitCode,
                    maxCPU: statusData.max_cpu || statusData.maxCPU || 0,
                    maxMemory: statusData.max_memory || statusData.maxMemory || 0,
                    maxIOBPS: statusData.max_io_bps || statusData.maxIOBPS || 0,
                    cpuCores: statusData.cpu_cores || statusData.cpuCores,
                    runtime: statusData.runtime,
                    network: statusData.network || 'bridge',
                    volumes: statusData.volumes || [],
                    uploads: statusData.uploads || statusData.uploaded_files || [],
                    uploadDirs: statusData.upload_dirs || statusData.uploadDirs || [],
                    workingDir: statusData.working_dir || statusData.workingDir,
                    envVars: statusData.environment || statusData.envVars || {},
                    secretEnvVars: statusData.secrets || statusData.secretEnvVars || {},
                    dependsOn: statusData.depends_on || statusData.dependsOn || [],
                    workflowUUID: statusData.workflow_uuid || statusData.workflowUUID
                };
                setSelectedJob(enhancedJob);
            } catch (statusError) {
                // Fallback to regular job details if status endpoint fails
                console.warn('Failed to fetch comprehensive status, falling back to basic details:', statusError);
                const jobDetails = await apiService.getJob(jobId);
                setSelectedJob(jobDetails);
            }
        } catch (error) {
            console.error('Failed to fetch job details:', error);
        } finally {
            setJobLoading(false);
        }
    };

    const handleStopJob = (jobId: string) => {
        setStopJobConfirm({show: true, jobId, stopping: false});
    };

    const confirmStopJob = async () => {
        if (!stopJobConfirm.jobId) return;

setStopJobConfirm(prev => ({...prev, stopping: true}));
        setStoppingJobId(stopJobConfirm.jobId);
        try {
            await stopJob(stopJobConfirm.jobId);
            setStopJobConfirm({show: false, jobId: '', stopping: false});
        } catch (error) {
            console.error('Failed to stop job:', error);
            alert('Failed to stop job: ' + (error instanceof Error ? error.message : 'Unknown error'));
            setStopJobConfirm(prev => ({...prev, stopping: false}));
        } finally {
            setStoppingJobId(null);
        }
    };

    const cancelStopJob = () => {
        setStopJobConfirm({show: false, jobId: '', stopping: false});
    };

    const handleDeleteAllJobs = () => {
        setDeleteAllConfirm({show: true, deleting: false});
    };

    const confirmDeleteAllJobs = async () => {
        setDeleteAllConfirm(prev => ({...prev, deleting: true}));
        try {
            await deleteAllJobs();
            setDeleteAllConfirm({show: false, deleting: false});
        } catch (error) {
            console.error('Failed to delete all jobs:', error);
            alert('Failed to delete all jobs: ' + (error instanceof Error ? error.message : 'Unknown error'));
            setDeleteAllConfirm(prev => ({...prev, deleting: false}));
        }
    };

    const cancelDeleteAllJobs = () => {
        setDeleteAllConfirm({show: false, deleting: false});
    };

    const handleDeleteJob = (jobId: string) => {
        setDeleteJobConfirm({show: true, jobId, deleting: false});
    };

    const confirmDeleteJob = async () => {
        if (!deleteJobConfirm.jobId) return;

        setDeleteJobConfirm(prev => ({...prev, deleting: true}));
        setDeletingJobId(deleteJobConfirm.jobId);
        try {
            await deleteJob(deleteJobConfirm.jobId);
            setDeleteJobConfirm({show: false, jobId: '', deleting: false});
        } catch (error) {
            console.error('Failed to delete job:', error);
            alert('Failed to delete job: ' + (error instanceof Error ? error.message : 'Unknown error'));
            setDeleteJobConfirm(prev => ({...prev, deleting: false}));
        } finally {
            setDeletingJobId(null);
        }
    };

    const cancelDeleteJob = () => {
        setDeleteJobConfirm({show: false, jobId: '', deleting: false});
    };

    const handleCloseModal = () => {
        setSelectedJobId(null);
        setSelectedJob(null);
        setActiveTab('logs');
        clearLogs();
    };

    // Handle escape key to close modal
    useEffect(() => {
        const handleEscapeKey = (event: KeyboardEvent) => {
            if (event.key === 'Escape' && selectedJobId) {
                handleCloseModal();
            }
        };

        document.addEventListener('keydown', handleEscapeKey);
        return () => {
            document.removeEventListener('keydown', handleEscapeKey);
        };
    }, [selectedJobId]);

    // Handle escape key to close create job dialog
    useEffect(() => {
        const handleEscapeKey = (event: KeyboardEvent) => {
            if (event.key === 'Escape' && showCreateJob) {
                handleCloseCreateJob();
            }
        };

        document.addEventListener('keydown', handleEscapeKey);
        return () => {
            document.removeEventListener('keydown', handleEscapeKey);
        };
    }, [showCreateJob]);

    // Auto-scroll to bottom when new logs arrive
    useEffect(() => {
        if (autoScroll && logContainerRef.current) {
            logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
        }
    }, [logs, autoScroll]);

    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">Jobs</h1>
                        <p className="mt-2 text-gray-300">Manage and monitor job execution</p>
                        <div className="mt-2 flex items-center text-sm">
                            <div className="w-2 h-2 rounded-full mr-2 bg-green-500 animate-pulse"></div>
                            <span className="text-gray-400">Auto-refresh enabled (5s)</span>
                        </div>
                    </div>
                    <div className="flex space-x-3">
                        {totalJobs > 0 && (
                            <button
                                onClick={handleDeleteAllJobs}
                                className="inline-flex items-center px-4 py-2 border border-red-600 rounded-md shadow-sm text-sm font-medium text-red-300 bg-transparent hover:bg-red-600 hover:text-white"
                                title="Delete all non-running jobs"
                            >
                                <Trash2 className="h-4 w-4 mr-2"/>
                                Delete All Jobs
                            </button>
                        )}
                        <button
                            onClick={() => setShowCreateJob(true)}
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
                        >
                            <Plus className="h-4 w-4 mr-2"/>
                            {t('jobs.newJob')}
                        </button>
                    </div>
                </div>
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-white">{t('jobs.loadingJobs')}</p>
                    </div>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-red-500">{t('common.error')}: {error}</p>
                    </div>
                </div>
            ) : (
                <div className="bg-gray-800 rounded-lg shadow overflow-hidden">
                    <div className="px-6 py-4 border-b border-gray-200">
                        <div className="flex items-center justify-between">
                            <h3 className="text-lg font-medium text-white">
                                {t('jobs.title')} ({totalJobs})
                            </h3>
                            <div className="flex items-center space-x-4">
                                <div className="flex items-center space-x-2">
                                    <label className="text-sm text-gray-300">Show:</label>
                                    <select
                                        value={pageSize}
                                        onChange={(e) => setPageSize(Number(e.target.value))}
                                        className="bg-gray-700 text-white border border-gray-600 rounded px-2 py-1 text-sm"
                                    >
                                        <option value={5}>5</option>
                                        <option value={10}>10</option>
                                        <option value={25}>25</option>
                                        <option value={50}>50</option>
                                        <option value={100}>100</option>
                                    </select>
                                    <span className="text-sm text-gray-300">per page</span>
                                </div>
                                <div className="text-sm text-gray-300">
                                    Showing {totalJobs === 0 ? 0 : (currentPage - 1) * pageSize + 1}-{Math.min(currentPage * pageSize, totalJobs)} of {totalJobs}
                                </div>
                            </div>
                        </div>
                    </div>

                    {totalJobs === 0 ? (
                        <div className="p-6 text-center">
                            <p className="text-gray-500">{t('jobs.noJobs')}</p>
                            <p className="text-sm text-gray-400 mt-1">Create your first job to get started</p>
                        </div>
                    ) : (
                        <>
                            <div className="overflow-x-auto">
                                <table className="min-w-full divide-y divide-gray-200">
                                    <thead className="bg-auto">
                                    <tr>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Job
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Status
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Command
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Duration
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Started
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-white uppercase tracking-wider">
                                            Actions
                                        </th>
                                    </tr>
                                    </thead>
                                    <tbody className="bg-auto divide-y divide-gray-200">
                                    {paginatedJobs.map((job) => (
                                        <tr key={job.id} className="hover:bg-gray-700">
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div>
                                                    <div className="text-sm text-white font-mono" title={job.id}>
                                                        {shortenUuid(job.id)}
                                                    </div>
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                        <span
                            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(job.status)}`}>
                          {job.status}
                        </span>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div className="text-sm text-white max-w-xs truncate">
                                                    {job.command} {job.args?.join(' ') || ''}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-white">
                                                {(job.startTime && job.endTime) ?
                                                    formatDuration(new Date(job.endTime).getTime() - new Date(job.startTime).getTime()) : '-'}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-white">
                                                {job.startTime ? new Date(job.startTime).toLocaleString() : '-'}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                                                <div className="flex space-x-2">
                                                    <button
                                                        onClick={() => handleViewJob(job.id)}
                                                        className="text-green-600 hover:text-green-300"
                                                        title="View Job Details & Logs"
                                                    >
                                                        <FileText className="h-4 w-4"/>
                                                    </button>
                                                    {job.status === 'RUNNING' && (
                                                        <button
                                                            onClick={() => handleStopJob(job.id)}
                                                            disabled={stoppingJobId === job.id}
                                                            className="text-red-600 hover:text-red-900 disabled:opacity-50 disabled:cursor-not-allowed"
                                                            title={stoppingJobId === job.id ? "Stopping..." : "Stop Job"}
                                                        >
                                                            <Square className="h-4 w-4"/>
                                                        </button>
                                                    )}
                                                    {(job.status === 'QUEUED' || job.status === 'PENDING') && (
                                                        <button className="text-blue-600 hover:text-blue-300">
                                                            <Play className="h-4 w-4"/>
                                                        </button>
                                                    )}
                                                    {(job.status === 'COMPLETED' || job.status === 'FAILED' || job.status === 'STOPPED') && (
                                                        <button
                                                            onClick={() => handleDeleteJob(job.id)}
                                                            disabled={deletingJobId === job.id}
                                                            className="text-red-500 hover:text-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                                            title={deletingJobId === job.id ? "Deleting..." : "Delete Job"}
                                                        >
                                                            <Trash2 className="h-4 w-4"/>
                                                        </button>
                                                    )}
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                    </tbody>
                                </table>
                            </div>

                            {/* Pagination Controls */}
                            {totalPages > 1 && (
                                <div className="px-6 py-4 border-t border-gray-700">
                                    <div className="flex items-center justify-between">
                                        <div className="text-sm text-gray-300">
                                            Page {currentPage} of {totalPages}
                                        </div>
                                        <div className="flex items-center space-x-1">
                                            <button
                                                onClick={() => setCurrentPage(currentPage - 1)}
                                                disabled={currentPage === 1}
                                                className="px-3 py-1 border border-gray-600 rounded text-sm text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
                                            >
                                                <ChevronLeft className="h-4 w-4 mr-1"/>
                                                Previous
                                            </button>

                                            {/* Page Numbers */}
                                            <div className="flex items-center space-x-1">
                                                {Array.from({length: Math.min(totalPages, 5)}, (_, i) => {
                                                    let pageNum: number;
                                                    if (totalPages <= 5) {
                                                        pageNum = i + 1;
                                                    } else if (currentPage <= 3) {
                                                        pageNum = i + 1;
                                                    } else if (currentPage >= totalPages - 2) {
                                                        pageNum = totalPages - 4 + i;
                                                    } else {
                                                        pageNum = currentPage - 2 + i;
                                                    }

                                                    return (
                                                        <button
                                                            key={pageNum}
                                                            onClick={() => setCurrentPage(pageNum)}
                                                            className={`px-3 py-1 border rounded text-sm ${
                                                                currentPage === pageNum
                                                                    ? 'bg-blue-600 text-white border-blue-600'
                                                                    : 'border-gray-600 text-gray-300 hover:bg-gray-700'
                                                            }`}
                                                        >
                                                            {pageNum}
                                                        </button>
                                                    );
                                                })}
                                            </div>

                                            <button
                                                onClick={() => setCurrentPage(currentPage + 1)}
                                                disabled={currentPage === totalPages}
                                                className="px-3 py-1 border border-gray-600 rounded text-sm text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
                                            >
                                                Next
                                                <ChevronRight className="h-4 w-4 ml-1"/>
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            )}

                            {/* Command Preview Section */}
                            <div className="mt-6 bg-gray-800 border border-gray-700 rounded-lg p-6">
                                <h3 className="text-lg font-medium text-gray-200 mb-4">Command Examples</h3>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Run Simple Job
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-green-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job run "echo Hello World"
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Run with Runtime
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-green-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job run "python3 script.py" --runtime=python-3.11
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            List Jobs
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-green-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job list
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Stop Job
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-red-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job stop &lt;job-id&gt;
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            View Job Logs
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-blue-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job logs &lt;job-id&gt;
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Run with Resources
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-green-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job run "npm test" --cpu=50 --memory=512MB
                                        </pre>
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-300 mb-2">
                                            Delete All Non-Running Jobs
                                        </label>
                                        <pre
                                            className="bg-gray-900 text-red-400 p-3 rounded-md text-sm overflow-x-auto font-mono">
rnx job delete-all
                                        </pre>
                                    </div>
                                </div>
                            </div>
                        </>
                    )}
                </div>
            )}

            {/* Log Modal */}
            {selectedJobId && (
                <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
                    <div
                        className="relative top-16 mx-auto p-5 border w-11/12 max-w-[90vw] min-h-[80vh] shadow-lg rounded-md bg-white dark:bg-gray-800">
                        <div className="flex items-center justify-between pb-3 border-b">
                            <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                                Job Details - {selectedJobId}
                            </h3>
                            <button
                                onClick={handleCloseModal}
                                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                            >
                                <X className="h-5 w-5"/>
                            </button>
                        </div>

                        {/* Tab Navigation */}
                        <div className="border-b border-gray-200 dark:border-gray-600">
                            <nav className="flex space-x-8">
                                <button
                                    onClick={() => setActiveTab('logs')}
                                    className={`py-2 px-1 border-b-2 font-medium text-sm ${
                                        activeTab === 'logs'
                                            ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                                            : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
                                    }`}
                                >
                                    Logs
                                </button>
                                <button
                                    onClick={() => setActiveTab('details')}
                                    className={`py-2 px-1 border-b-2 font-medium text-sm ${
                                        activeTab === 'details'
                                            ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                                            : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
                                    }`}
                                >
                                    Details
                                </button>
                            </nav>
                        </div>

                        <div className="py-4">
                            {/* Logs Tab */}
                            {activeTab === 'logs' && (
                                <>
                                    <div className="flex items-center justify-between mb-4">
                                        <div className="flex items-center space-x-4">
                                            <div className="flex items-center space-x-2">
                                                <div
                                                    className={`w-3 h-3 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}></div>
                                                <span className="text-sm text-gray-600 dark:text-gray-400">
                                                    {connected ? t('jobs.connected') : t('jobs.disconnected')}
                                                </span>
                                            </div>
                                            <label
                                                className="flex items-center space-x-2 text-sm text-gray-600 dark:text-gray-400">
                                                <input
                                                    type="checkbox"
                                                    checked={autoScroll}
                                                    onChange={(e) => setAutoScroll(e.target.checked)}
                                                    className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                                                />
                                                <span>Auto-scroll</span>
                                            </label>
                                        </div>
                                        <button
                                            onClick={clearLogs}
                                            className="px-3 py-1 text-sm bg-gray-500 hover:bg-gray-600 text-white rounded"
                                        >
                                            Clear Logs
                                        </button>
                                    </div>

                                    {logError && (
                                        <div
                                            className="mb-4 p-3 bg-red-100 dark:bg-red-900 border border-red-400 text-red-700 dark:text-red-300 rounded">
                                            {t('common.error')}: {logError}
                                        </div>
                                    )}

                                    <div
                                        ref={logContainerRef}
                                        className="bg-black text-green-400 p-4 rounded-lg h-[70vh] overflow-y-auto font-mono text-sm"
                                    >
                                        {logs.length === 0 ? (
                                            <div className="text-gray-500">No logs available yet...</div>
                                        ) : (
                                            logs.map((log, index) => (
                                                <div key={index} className={`mb-1 whitespace-pre-wrap ${
                                                    log.type === 'system' ? 'text-gray-400 opacity-80' :
                                                        log.type === 'info' ? 'text-gray-200' :
                                                            log.type === 'error' ? 'text-red-400' :
                                                                log.type === 'connection' ? 'text-blue-400' :
                                                                    'text-green-400'
                                                }`}>
                                                    {log.message}
                                                </div>
                                            ))
                                        )}
                                    </div>
                                </>
                            )}

                            {/* Details Tab */}
                            {activeTab === 'details' && (
                                <div className="space-y-6">
                                    {jobLoading ? (
                                        <div className="flex items-center justify-center py-8">
                                            <div
                                                className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
                                            <span
                                                className="ml-3 text-gray-600 dark:text-gray-400">{t('jobs.loadingJobDetails')}</span>
                                        </div>
                                    ) : selectedJob ? (
                                        <>
                                            {/* Basic Information */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Basic
                                                    Information</h4>
                                                <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Job
                                                            ID
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">{selectedJob.id}</dd>
                                                    </div>
                                                    {selectedJob.name && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Job Name</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.name}</dd>
                                                        </div>
                                                    )}
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Status</dt>
                                                        <dd className="mt-1">
                                                            <span
                                                                className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${getStatusColor(selectedJob.status)}`}>
                                                                {selectedJob.status}
                                                            </span>
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Command</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">{selectedJob.command}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Arguments</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                                                            {selectedJob.args?.length ? selectedJob.args.join(' ') : 'None'}
                                                        </dd>
                                                    </div>
                                                    {selectedJob.workingDir && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Working Directory</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">{selectedJob.workingDir}</dd>
                                                        </div>
                                                    )}
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Duration</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{formatDuration(selectedJob.duration)}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Exit
                                                            Code
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.exitCode ?? 'N/A'}</dd>
                                                    </div>
                                                    {selectedJob.scheduledTime && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Scheduled
                                                                Time
                                                            </dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                                {new Date(selectedJob.scheduledTime).toLocaleString()}
                                                                <div
                                                                    className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                                                    {(selectedJob.status === 'QUEUED' || selectedJob.status === 'PENDING') && (
                                                                        <span
                                                                            className="text-blue-600 dark:text-blue-400">
                                                                            {t('jobs.waitingToRun')}
                                                                        </span>
                                                                    )}
                                                                </div>
                                                            </dd>
                                                        </div>
                                                    )}
                                                </dl>
                                            </div>

                                            {/* Timing Information */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Timing Information</h4>
                                                <dl className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
                                                    {selectedJob.createdTime && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Created Time</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                                {new Date(selectedJob.createdTime).toLocaleString()}
                                                            </dd>
                                                        </div>
                                                    )}
                                                    {selectedJob.startTime && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Start Time</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                                {new Date(selectedJob.startTime).toLocaleString()}
                                                            </dd>
                                                        </div>
                                                    )}
                                                    {selectedJob.endTime && (
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">End Time</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                                {new Date(selectedJob.endTime).toLocaleString()}
                                                            </dd>
                                                        </div>
                                                    )}
                                                </dl>
                                            </div>

                                            {/* Resource Limits */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Resource
                                                    Limits</h4>
                                                <dl className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            CPU (%)
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.maxCPU || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            Memory (MB)
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.maxMemory || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            IO BPS
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.maxIOBPS || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">CPU
                                                            Cores
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.cpuCores || 'Default'}</dd>
                                                    </div>
                                                </dl>
                                            </div>

                                            {/* Configuration */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Configuration</h4>
                                                <dl className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Runtime</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.runtime || 'Default'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Network</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJob.network || 'Default'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Volumes</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                            {selectedJob.volumes?.length ? (
                                                                <div className="space-y-1">
                                                                    {selectedJob.volumes.map((volume, index) => (
                                                                        <div key={index} className="text-sm bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded inline-block mr-1 mb-1">
                                                                            {volume}
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            ) : 'None'}
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Uploads</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                            {selectedJob.uploads?.length ? (
                                                                <div className="space-y-1">
                                                                    {selectedJob.uploads.map((upload, index) => (
                                                                        <div key={index} className="text-sm bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded inline-block mr-1 mb-1">
                                                                            {upload}
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            ) : 'None'}
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Upload Directories</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                            {selectedJob.uploadDirs?.length ? (
                                                                <div className="space-y-1">
                                                                    {selectedJob.uploadDirs.map((uploadDir, index) => (
                                                                        <div key={index} className="text-sm bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded inline-block mr-1 mb-1">
                                                                            {uploadDir}
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            ) : 'None'}
                                                        </dd>
                                                    </div>
                                                </dl>
                                            </div>

                                            {/* Workflow Context */}
                                            {selectedJob.workflowUUID && (
                                                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                    <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Workflow Context</h4>
                                                    <dl className="grid grid-cols-1 gap-4">
                                                        <div>
                                                            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Workflow UUID</dt>
                                                            <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                                                                {selectedJob.workflowUUID}
                                                            </dd>
                                                        </div>
                                                    </dl>
                                                </div>
                                            )}

                                            {/* Dependencies */}
                                            {selectedJob.dependsOn && selectedJob.dependsOn.length > 0 && (
                                                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                    <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Dependencies</h4>
                                                    <div className="space-y-2">
                                                        {selectedJob.dependsOn.map((dependency, index) => {
                                                            // Parse dependency format: "job-id:COMPLETED" or just "job-id"
                                                            const [jobId, condition] = dependency.includes(':')
                                                                ? dependency.split(':')
                                                                : [dependency, 'COMPLETED'];

                                                            return (
                                                                <div key={index}
                                                                     className="flex items-center space-x-3 p-3 bg-white dark:bg-gray-600 rounded border">
                                                                    <div className="flex-1">
                                                                        <div
                                                                            className="text-sm font-medium text-gray-900 dark:text-white">
                                                                            Depends on: <span
                                                                            className="font-mono">{jobId}</span>
                                                                        </div>
                                                                        <div
                                                                            className="text-xs text-gray-500 dark:text-gray-400">
                                                                            Required condition: <span
                                                                            className={`font-mono px-2 py-1 rounded text-xs ${
                                                                                condition === 'COMPLETED' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300' :
                                                                                    condition === 'FAILED' ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300' :
                                                                                        'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
                                                                            }`}>{condition}</span>
                                                                        </div>
                                                                    </div>
                                                                </div>
                                                            );
                                                        })}
                                                    </div>
                                                </div>
                                            )}

                                            {/* Environment Variables */}
                                            {(Object.keys(selectedJob.envVars || {}).length > 0 || Object.keys(selectedJob.secretEnvVars || {}).length > 0) && (
                                                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                    <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Environment
                                                        Variables</h4>

                                                    {Object.keys(selectedJob.envVars || {}).length > 0 && (
                                                        <div className="mb-4">
                                                            <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Regular
                                                                Variables</h5>
                                                            <div className="space-y-1">
                                                                {Object.entries(selectedJob.envVars || {}).map(([key, value]) => (
                                                                    <div key={key} className="flex text-sm">
                                                                        <span
                                                                            className="font-mono text-gray-600 dark:text-gray-400 w-32 flex-shrink-0">{key}=</span>
                                                                        <span
                                                                            className="font-mono text-gray-900 dark:text-white break-all">{value}</span>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        </div>
                                                    )}

                                                    {Object.keys(selectedJob.secretEnvVars || {}).length > 0 && (
                                                        <div>
                                                            <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Secret
                                                                Variables</h5>
                                                            <div className="space-y-1">
                                                                {Object.keys(selectedJob.secretEnvVars || {}).map((key) => (
                                                                    <div key={key} className="flex text-sm">
                                                                        <span
                                                                            className="font-mono text-gray-600 dark:text-gray-400 w-32 flex-shrink-0">{key}=</span>
                                                                        <span
                                                                            className="font-mono text-yellow-600 dark:text-yellow-400">*** (secret)</span>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        </div>
                                                    )}
                                                </div>
                                            )}
                                        </>
                                    ) : (
                                        <div className="text-center py-8">
                                            <p className="text-gray-500 dark:text-gray-400">Failed to load job
                                                details</p>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}

            {/* Stop Job Confirmation Dialog */}
            {stopJobConfirm.show && (
                <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    Stop Job
                                </h3>
                                <button
                                    onClick={cancelStopJob}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={stopJobConfirm.stopping}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        Are you sure you want to stop job "{stopJobConfirm.jobId}"?
                                    </p>
                                    <p className="text-sm text-orange-400">
                                        This will terminate the running job immediately. Any unsaved work may be lost.
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre className="bg-gray-900 text-orange-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{`rnx job stop ${stopJobConfirm.jobId}`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelStopJob}
                                    disabled={stopJobConfirm.stopping}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmStopJob}
                                    disabled={stopJobConfirm.stopping}
                                    className="px-4 py-2 bg-orange-600 hover:bg-orange-700 disabled:bg-orange-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {stopJobConfirm.stopping ? (
                                        <>
                                            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Stopping...
                                        </>
                                    ) : (
                                        <>
                                            <Square className="h-4 w-4 mr-2"/>
                                            Stop Job
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Create Job Dialog */}
            {showCreateJob && (
                <div
                    className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div
                        className="relative bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-hidden">
                        <div className="flex items-center justify-between p-6 border-b border-gray-600">
                            <h3 className="text-lg font-medium text-gray-200">{t('jobs.createNew')}</h3>
                            <button
                                onClick={handleCloseCreateJob}
                                className="text-gray-400 hover:text-gray-300"
                            >
                                <X className="h-5 w-5"/>
                            </button>
                        </div>
                        <div className="overflow-y-auto max-h-[calc(90vh-80px)]">
                            <SimpleJobBuilder
                                onJobCreated={handleJobCreated}
                                onClose={handleCloseCreateJob}
                                showHeader={false}
                            />
                        </div>
                    </div>
                </div>
            )}

            {/* Delete All Jobs Confirmation Dialog */}
            {deleteAllConfirm.show && (
                <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    Delete All Jobs
                                </h3>
                                <button
                                    onClick={cancelDeleteAllJobs}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={deleteAllConfirm.deleting}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        Are you sure you want to delete all non-running jobs?
                                    </p>
                                    <p className="text-sm text-red-400">
                                        This will permanently delete all completed, failed, and stopped jobs including their logs and metadata. Running and scheduled jobs will not be affected.
                                    </p>
                                    <p className="text-sm text-orange-400 mt-2">
                                        This action cannot be UNDONE.
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre className="bg-gray-900 text-red-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{`rnx job delete-all`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelDeleteAllJobs}
                                    disabled={deleteAllConfirm.deleting}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmDeleteAllJobs}
                                    disabled={deleteAllConfirm.deleting}
                                    className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {deleteAllConfirm.deleting ? (
                                        <>
                                            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Deleting...
                                        </>
                                    ) : (
                                        <>
                                            <Trash2 className="h-4 w-4 mr-2"/>
                                            Delete All Jobs
                                        </>
                                    )}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Delete Job Confirmation Dialog */}
            {deleteJobConfirm.show && (
                <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50 flex items-center justify-center">
                    <div className="relative bg-gray-800 rounded-lg shadow-xl max-w-lg w-full mx-4">
                        <div className="p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="text-lg font-medium text-gray-200">
                                    Delete Job
                                </h3>
                                <button
                                    onClick={cancelDeleteJob}
                                    className="text-gray-400 hover:text-gray-300"
                                    disabled={deleteJobConfirm.deleting}
                                >
                                    <X className="h-5 w-5"/>
                                </button>
                            </div>

                            <div className="space-y-4">
                                <div>
                                    <p className="text-gray-300 mb-2">
                                        Are you sure you want to delete job "{deleteJobConfirm.jobId}"?
                                    </p>
                                    <p className="text-sm text-red-400">
                                        This will permanently delete the job including its logs and metadata.
                                    </p>
                                    <p className="text-sm text-orange-400 mt-2">
                                        This action cannot be UNDONE.
                                    </p>
                                </div>

                                {/* Command Preview */}
                                <div>
                                    <label className="block text-sm font-medium text-gray-300 mb-2">
                                        Command Preview
                                    </label>
                                    <pre className="bg-gray-900 text-red-400 p-4 rounded-md text-sm overflow-x-auto font-mono">
{`rnx job delete ${deleteJobConfirm.jobId}`}
                                    </pre>
                                </div>
                            </div>

                            <div className="flex space-x-3 justify-end mt-6">
                                <button
                                    onClick={cancelDeleteJob}
                                    disabled={deleteJobConfirm.deleting}
                                    className="px-4 py-2 border border-gray-600 rounded-md text-sm font-medium text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={confirmDeleteJob}
                                    disabled={deleteJobConfirm.deleting}
                                    className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white rounded-md text-sm font-medium disabled:cursor-not-allowed flex items-center"
                                >
                                    {deleteJobConfirm.deleting ? (
                                        <>
                                            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                                            Deleting...
                                        </>
                                    ) : (
                                        <>
                                            <Trash2 className="h-4 w-4 mr-2"/>
                                            Delete Job
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

export default Jobs;