import React from 'react';
import {Calendar, ChevronLeft, ChevronRight, Clock, Network, Play, Square} from 'lucide-react';

interface WorkflowListProps {
    workflows: Array<{
        id: string | number; // Support both UUID strings and legacy numeric IDs
        name: string;
        workflow: string;
        status: 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';
        total_jobs: number;
        completed_jobs: number;
        failed_jobs: number;
        created_at: string;
        started_at?: string;
        completed_at?: string;
    }>;
    onWorkflowClick: (workflowId: string) => void;
    loading?: boolean;
    // Pagination props
    currentPage?: number;
    pageSize?: number;
    totalWorkflows?: number;
    totalPages?: number;
    setCurrentPage?: (page: number) => void;
    setPageSize?: (size: number) => void;
}

const WorkflowList: React.FC<WorkflowListProps> = ({
                                                       workflows,
                                                       onWorkflowClick,
                                                       loading = false,
                                                       currentPage = 1,
                                                       pageSize = 10,
                                                       totalWorkflows,
                                                       totalPages = 1,
                                                       setCurrentPage,
                                                       setPageSize
                                                   }) => {
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


    if (loading) {
        return (
            <div className="bg-gray-800 rounded-lg shadow">
                <div className="p-6">
                    <p className="text-white">Loading workflows...</p>
                </div>
            </div>
        );
    }

    return (
        <div className="bg-gray-800 rounded-lg shadow overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-700">
                <div className="flex items-center justify-between">
                    <h3 className="text-lg font-medium text-white">
                        All Workflows ({totalWorkflows ?? workflows.length})
                    </h3>
                    {setPageSize && totalWorkflows !== undefined && (
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
                                </select>
                                <span className="text-sm text-gray-300">per page</span>
                            </div>
                            <div className="text-sm text-gray-300">
                                Showing {totalWorkflows === 0 ? 0 : (currentPage - 1) * pageSize + 1}-{Math.min(currentPage * pageSize, totalWorkflows)} of {totalWorkflows}
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {workflows.length === 0 ? (
                <div className="p-6 text-center">
                    <Network className="h-12 w-12 text-gray-400 mx-auto mb-4"/>
                    <p className="text-gray-500">No workflows found</p>
                    <p className="text-sm text-gray-400 mt-1">
                        Workflows with job dependencies will appear here
                    </p>
                </div>
            ) : (
                <div className="divide-y divide-gray-700">
                    {workflows.map((workflow, index) => (
                        <div
                            key={`workflow-${workflow.id || index}-${index}`}
                            onClick={() => {
                                if (workflow.id) {
                                    onWorkflowClick(workflow.id.toString());
                                }
                            }}
                            className="p-6 hover:bg-gray-700 cursor-pointer transition-colors"
                        >
                            <div className="flex items-center justify-between">
                                <div className="flex-1">
                                    <div className="flex items-center space-x-3">
                                        <h4 className="text-lg font-medium text-white">
                                            {workflow.name}
                                        </h4>
                                        <span
                                            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(workflow.status)}`}
                                        >
                                            {workflow.status}
                                        </span>
                                    </div>

                                    <p className="text-sm text-gray-300 mt-1">
                                        {workflow.workflow}
                                    </p>


                                    <div className="flex items-center space-x-6 mt-3">
                                        <div className="flex items-center text-sm text-gray-400">
                                            <Network className="h-4 w-4 mr-1"/>
                                            <span>{workflow.total_jobs} jobs ({workflow.completed_jobs} completed, {workflow.failed_jobs} failed)</span>
                                        </div>

                                        <div className="flex items-center text-sm text-gray-400">
                                            <Calendar className="h-4 w-4 mr-1"/>
                                            <span>Created: {new Date(workflow.created_at).toLocaleString()}</span>
                                        </div>

                                        {workflow.started_at && (
                                            <div className="flex items-center text-sm text-gray-400">
                                                <Clock className="h-4 w-4 mr-1"/>
                                                <span>Started: {new Date(workflow.started_at).toLocaleString()}</span>
                                            </div>
                                        )}
                                    </div>
                                </div>

                                <div className="flex items-center space-x-2 ml-4">
                                    {workflow.status === 'RUNNING' ? (
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                // TODO: Stop workflow
                                            }}
                                            className="p-2 text-red-400 hover:text-red-300 hover:bg-gray-600 rounded"
                                            title="Stop Workflow"
                                        >
                                            <Square className="h-4 w-4"/>
                                        </button>
                                    ) : (
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                // TODO: Start workflow
                                            }}
                                            className="p-2 text-green-400 hover:text-green-300 hover:bg-gray-600 rounded"
                                            title="Start Workflow"
                                        >
                                            <Play className="h-4 w-4"/>
                                        </button>
                                    )}
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Pagination Controls */}
            {setCurrentPage && totalPages && totalPages > 1 && (
                <div className="px-6 py-4 border-t border-gray-700">
                    <div className="flex items-center justify-between">
                        <div className="text-sm text-gray-300">
                            Page {currentPage} of {totalPages}
                        </div>
                        <div className="flex items-center space-x-1">
                            <button
                                onClick={() => setCurrentPage?.(currentPage - 1)}
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
                                            onClick={() => setCurrentPage?.(pageNum)}
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
                                onClick={() => setCurrentPage?.(currentPage + 1)}
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
        </div>
    );
};

export default WorkflowList;