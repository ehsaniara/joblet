import React, { useState } from 'react';
import { Job } from '../../types/job';
import { WorkflowGraph } from './WorkflowGraph';
import { ArrowLeft, BarChart3, List, Network, RotateCcw } from 'lucide-react';
import clsx from 'clsx';

interface WorkflowDetailProps {
    workflow: {
        id: string;
        name: string;
        description?: string;
        jobs: Job[];
        status: 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';
        lastRun?: string;
    };
    onBack: () => void;
    onRefresh: () => void;
}

type ViewMode = 'graph' | 'tree' | 'timeline';

const WorkflowDetail: React.FC<WorkflowDetailProps> = ({
    workflow,
    onBack,
    onRefresh
}) => {
    const [viewMode, setViewMode] = useState<ViewMode>('graph');
    const [selectedJob, setSelectedJob] = useState<Job | null>(null);

    const handleJobSelect = (job: Job | null) => {
        setSelectedJob(job);
    };

    const handleJobAction = (job: Job, action: string) => {
        if (action === 'details') {
            console.log('Show job details:', job);
            // TODO: Open job details modal
        }
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

    const viewModes = [
        { key: 'graph' as ViewMode, label: 'Graph View', icon: Network },
        { key: 'tree' as ViewMode, label: 'Tree View', icon: List },
        { key: 'timeline' as ViewMode, label: 'Timeline', icon: BarChart3 },
    ];

    return (
        <div className="flex flex-col h-full">
            {/* Header */}
            <div className="p-6 border-b border-gray-200">
                <div className="flex items-center space-x-4 mb-4">
                    <button
                        onClick={onBack}
                        className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                    >
                        <ArrowLeft className="w-4 h-4 mr-2" />
                        Back to Workflows
                    </button>
                    <div className="flex-1">
                        <div className="flex items-center space-x-3">
                            <h1 className="text-3xl font-bold text-white">{workflow.name}</h1>
                            <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(workflow.status)}`}
                            >
                                {workflow.status}
                            </span>
                        </div>
                        {workflow.description && (
                            <p className="mt-2 text-gray-300">{workflow.description}</p>
                        )}
                    </div>
                    <button
                        onClick={onRefresh}
                        className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                    >
                        <RotateCcw className="w-4 h-4 mr-2" />
                        Refresh
                    </button>
                </div>

                {/* View Mode Tabs */}
                <div className="border-b border-gray-200">
                    <nav className="-mb-px flex space-x-8">
                        {viewModes.map(({ key, label, icon: Icon }) => (
                            <button
                                key={key}
                                onClick={() => setViewMode(key)}
                                className={clsx(
                                    'py-2 px-1 border-b-2 font-medium text-sm whitespace-nowrap flex items-center',
                                    viewMode === key
                                        ? 'border-blue-500 text-blue-600'
                                        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                                )}
                            >
                                <Icon className="w-4 h-4 mr-2" />
                                {label}
                            </button>
                        ))}
                    </nav>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-hidden">
                {/* Graph View */}
                {viewMode === 'graph' && (
                    <WorkflowGraph
                        jobs={workflow.jobs}
                        onJobSelect={handleJobSelect}
                        onJobAction={handleJobAction}
                    />
                )}

                {/* Tree View */}
                {viewMode === 'tree' && (
                    <div className="p-6">
                        <div className="bg-white rounded-lg shadow">
                            <div className="p-6">
                                <h3 className="text-lg font-medium text-gray-900 mb-4">
                                    Workflow Execution Tree
                                </h3>
                                {workflow.jobs.length === 0 ? (
                                    <div className="text-center py-8">
                                        <List className="w-8 h-8 text-gray-400 mx-auto mb-2" />
                                        <p className="text-gray-500">No jobs found in this workflow</p>
                                    </div>
                                ) : (
                                    <div className="space-y-4">
                                        {workflow.jobs.map(job => (
                                            <div key={job.id} className="border rounded-lg p-4">
                                                <div className="flex items-center justify-between">
                                                    <div>
                                                        <h4 className="font-medium">{job.id}</h4>
                                                        <p className="text-sm text-gray-500">
                                                            {job.command} {job.args.join(' ')}
                                                        </p>
                                                        {job.dependsOn && job.dependsOn.length > 0 && (
                                                            <p className="text-xs text-gray-400 mt-1">
                                                                Depends on: {job.dependsOn.join(', ')}
                                                            </p>
                                                        )}
                                                        {(job as any).start_time && (
                                                            <p className="text-xs text-gray-400 mt-1">
                                                                Started: {new Date((job as any).start_time).toLocaleString()}
                                                            </p>
                                                        )}
                                                    </div>
                                                    <span className={clsx(
                                                        'px-2 py-1 rounded-full text-xs font-medium',
                                                        getStatusColor(job.status)
                                                    )}>
                                                        {job.status}
                                                    </span>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                )}

                {/* Timeline View */}
                {viewMode === 'timeline' && (
                    <div className="p-6">
                        <div className="bg-white rounded-lg shadow">
                            <div className="p-6">
                                <h3 className="text-lg font-medium text-gray-900 mb-4">Timeline View</h3>
                                {workflow.jobs.length === 0 ? (
                                    <div className="text-center py-8">
                                        <BarChart3 className="w-8 h-8 text-gray-400 mx-auto mb-2" />
                                        <p className="text-gray-500">No timeline data available</p>
                                    </div>
                                ) : (
                                    <div className="space-y-3">
                                        {workflow.jobs
                                            .filter(job => (job as any).start_time)
                                            .sort((a, b) => {
                                                const aTime = new Date((a as any).start_time).getTime();
                                                const bTime = new Date((b as any).start_time).getTime();
                                                return aTime - bTime;
                                            })
                                            .map((job, index) => {
                                                const startTime = (job as any).start_time;
                                                const endTime = (job as any).end_time;
                                                const duration = endTime ? 
                                                    new Date(endTime).getTime() - new Date(startTime).getTime() : 0;
                                                
                                                return (
                                                    <div key={job.id} className="flex items-center space-x-4 p-3 border rounded-lg">
                                                        <div className="w-8 text-center text-sm text-gray-500">
                                                            {index + 1}
                                                        </div>
                                                        <div className="flex-1">
                                                            <div className="flex items-center space-x-2">
                                                                <span className="font-medium">{job.id}</span>
                                                                <span className={clsx(
                                                                    'px-2 py-1 rounded-full text-xs font-medium',
                                                                    getStatusColor(job.status)
                                                                )}>
                                                                    {job.status}
                                                                </span>
                                                            </div>
                                                            <p className="text-sm text-gray-600 mt-1">
                                                                {job.command} {job.args.join(' ')}
                                                            </p>
                                                        </div>
                                                        <div className="text-right text-sm text-gray-500">
                                                            <div>{new Date(startTime).toLocaleTimeString()}</div>
                                                            {duration > 0 && (
                                                                <div className="text-xs">
                                                                    {Math.round(duration / 1000)}s
                                                                </div>
                                                            )}
                                                        </div>
                                                    </div>
                                                );
                                            })}
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                )}
            </div>

            {/* Status Bar */}
            <div className="border-t border-gray-200 px-6 py-3 bg-gray-50">
                <div className="flex items-center justify-between text-sm text-gray-600">
                    <div>
                        <span>{workflow.jobs.length} jobs in workflow</span>
                        {workflow.lastRun && (
                            <span className="ml-4">
                                Last run: {new Date(workflow.lastRun).toLocaleString()}
                            </span>
                        )}
                    </div>
                    {selectedJob && (
                        <div>
                            Selected: {selectedJob.id} ({selectedJob.status})
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default WorkflowDetail;