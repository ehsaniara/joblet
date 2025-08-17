import React, {useEffect, useRef, useState} from 'react';
import {Link} from 'react-router-dom';
import {useJobs} from '../hooks/useJobs';
import {useLogStream} from '../hooks/useLogStream';
import {ChevronLeft, ChevronRight, FileText, Play, Plus, RotateCcw, Square, X} from 'lucide-react';

const Jobs: React.FC = () => {
    const {
        loading,
        error,
        refreshJobs,
        currentPage,
        pageSize,
        totalJobs,
        totalPages,
        paginatedJobs,
        setCurrentPage,
        setPageSize
    } = useJobs();
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [autoScroll, setAutoScroll] = useState<boolean>(true);
    const {logs, connected, error: logError, clearLogs} = useLogStream(selectedJobId);
    const logContainerRef = useRef<HTMLDivElement>(null);

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
                    </div>
                    <div className="flex space-x-3">
                        <button
                            onClick={refreshJobs}
                            className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                        >
                            <RotateCcw className="h-4 w-4 mr-2"/>
                            Refresh
                        </button>
                        <Link
                            to="/jobs/create"
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
                        >
                            <Plus className="h-4 w-4 mr-2"/>
                            New Job
                        </Link>
                    </div>
                </div>
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-white">Loading jobs...</p>
                    </div>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-red-500">Error: {error}</p>
                    </div>
                </div>
            ) : (
                <div className="bg-gray-800 rounded-lg shadow overflow-hidden">
                    <div className="px-6 py-4 border-b border-gray-200">
                        <div className="flex items-center justify-between">
                            <h3 className="text-lg font-medium text-white">
                                All Jobs ({totalJobs})
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
                            <p className="text-gray-500">No jobs found</p>
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
                                                    <div className="text-sm text-white">
                                                        {job.id}
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
                                                    {job.command} {job.args.join(' ')}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-white">
                                                {((job as any).start_time && (job as any).end_time) ? 
                                                   formatDuration(new Date((job as any).end_time).getTime() - new Date((job as any).start_time).getTime()) : '-'}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-white">
                                                {(job as any).start_time ? new Date((job as any).start_time).toLocaleString() : '-'}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                                                <div className="flex space-x-2">
                                                    <button
                                                        onClick={() => setSelectedJobId(job.id)}
                                                        className="text-green-600 hover:text-green-300"
                                                        title="View Logs"
                                                    >
                                                        <FileText className="h-4 w-4"/>
                                                    </button>
                                                    {job.status === 'RUNNING' ? (
                                                        <button className="text-red-600 hover:text-red-900">
                                                            <Square className="h-4 w-4"/>
                                                        </button>
                                                    ) : (
                                                        <button className="text-blue-600 hover:text-blue-300">
                                                            <Play className="h-4 w-4"/>
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
                                                    let pageNum;
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
                        </>
                    )}
                </div>
            )}

            {/* Log Modal */}
            {selectedJobId && (
                <div className="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50">
                    <div
                        className="relative top-20 mx-auto p-5 border w-11/12 max-w-4xl shadow-lg rounded-md bg-white dark:bg-gray-800">
                        <div className="flex items-center justify-between pb-3 border-b">
                            <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                                Job Logs - {selectedJobId}
                            </h3>
                            <button
                                onClick={() => {
                                    setSelectedJobId(null);
                                    clearLogs();
                                }}
                                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                            >
                                <X className="h-5 w-5"/>
                            </button>
                        </div>

                        <div className="py-4">
                            <div className="flex items-center justify-between mb-4">
                                <div className="flex items-center space-x-4">
                                    <div className="flex items-center space-x-2">
                                        <div
                                            className={`w-3 h-3 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}></div>
                                        <span className="text-sm text-gray-600 dark:text-gray-400">
                                            {connected ? 'Connected' : 'Disconnected'}
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
                                    Error: {logError}
                                </div>
                            )}

                            <div
                                ref={logContainerRef}
                                className="bg-black text-green-400 p-4 rounded-lg h-96 overflow-y-auto font-mono text-sm"
                            >
                                {logs.length === 0 ? (
                                    <div className="text-gray-500">No logs available yet...</div>
                                ) : (
                                    logs.map((log, index) => (
                                        <div key={index} className="mb-1 whitespace-pre-wrap">
                                            {log}
                                        </div>
                                    ))
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Jobs;