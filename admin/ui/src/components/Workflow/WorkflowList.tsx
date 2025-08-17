import React from 'react';
import { Job } from '../../types/job';
import { Calendar, Clock, Network, Play, Square } from 'lucide-react';

interface WorkflowListProps {
    workflows: Array<{
        id: string;
        name: string;
        description?: string;
        jobs: Job[];
        status: 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';
        lastRun?: string;
        duration?: number;
    }>;
    onWorkflowClick: (workflowId: string) => void;
    loading?: boolean;
}

const WorkflowList: React.FC<WorkflowListProps> = ({
    workflows,
    onWorkflowClick,
    loading = false
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

    const formatDuration = (duration?: number) => {
        if (!duration) return '-';
        const seconds = Math.floor(duration / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `${hours}h ${minutes % 60}m`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds % 60}s`;
        } else {
            return `${seconds}s`;
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
                <h3 className="text-lg font-medium text-white">
                    All Workflows ({workflows.length})
                </h3>
            </div>

            {workflows.length === 0 ? (
                <div className="p-6 text-center">
                    <Network className="h-12 w-12 text-gray-400 mx-auto mb-4" />
                    <p className="text-gray-500">No workflows found</p>
                    <p className="text-sm text-gray-400 mt-1">
                        Workflows with job dependencies will appear here
                    </p>
                </div>
            ) : (
                <div className="divide-y divide-gray-700">
                    {workflows.map((workflow) => (
                        <div
                            key={workflow.id}
                            onClick={() => onWorkflowClick(workflow.id)}
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
                                    
                                    {workflow.description && (
                                        <p className="text-sm text-gray-300 mt-1">
                                            {workflow.description}
                                        </p>
                                    )}

                                    <div className="flex items-center space-x-6 mt-3">
                                        <div className="flex items-center text-sm text-gray-400">
                                            <Network className="h-4 w-4 mr-1" />
                                            <span>{workflow.jobs.length} jobs</span>
                                        </div>
                                        
                                        {workflow.lastRun && (
                                            <div className="flex items-center text-sm text-gray-400">
                                                <Calendar className="h-4 w-4 mr-1" />
                                                <span>{new Date(workflow.lastRun).toLocaleString()}</span>
                                            </div>
                                        )}
                                        
                                        <div className="flex items-center text-sm text-gray-400">
                                            <Clock className="h-4 w-4 mr-1" />
                                            <span>{formatDuration(workflow.duration)}</span>
                                        </div>
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
                                            <Square className="h-4 w-4" />
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
                                            <Play className="h-4 w-4" />
                                        </button>
                                    )}
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};

export default WorkflowList;