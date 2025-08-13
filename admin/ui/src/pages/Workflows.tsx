import React, {useMemo, useState} from 'react';
import {useJobs} from '../hooks/useJobs';
import {WorkflowGraph} from '../components/Workflow/WorkflowGraph';
import {Job} from '../types/job';
import {BarChart3, List, Network, RotateCcw} from 'lucide-react';
import clsx from 'clsx';

type ViewMode = 'graph' | 'tree' | 'gantt';

const Workflows: React.FC = () => {
    const {jobs, loading, error, refreshJobs} = useJobs();
    const [viewMode, setViewMode] = useState<ViewMode>('graph');
    const [selectedJob, setSelectedJob] = useState<Job | null>(null);

    // Filter jobs that have dependencies or are dependencies (workflow jobs)
    const workflowJobs = useMemo(() => {
        const jobsWithDeps = jobs.filter(job =>
            (job.dependsOn && job.dependsOn.length > 0) ||
            jobs.some(otherJob => otherJob.dependsOn?.includes(job.id))
        );

        // If no jobs have dependencies, show recent jobs as example workflow
        if (jobsWithDeps.length === 0 && jobs.length > 0) {
            // Create mock dependencies for demonstration
            return jobs.slice(0, 4).map((job, index) => ({
                ...job,
                dependsOn: index > 0 ? [jobs[index - 1].id] : []
            }));
        }

        return jobsWithDeps;
    }, [jobs]);

    const handleJobSelect = (job: Job | null) => {
        setSelectedJob(job);
    };

    const handleJobAction = (job: Job, action: string) => {
        if (action === 'details') {
            console.log('Show job details:', job);
            // TODO: Open job details modal
        }
    };

    const viewModes = [
        {key: 'graph' as ViewMode, label: 'Graph View', icon: Network},
        {key: 'tree' as ViewMode, label: 'Tree View', icon: List},
        {key: 'gantt' as ViewMode, label: 'Timeline', icon: BarChart3},
    ];

    return (
        <div className="flex flex-col h-full">
            {/* Header */}
            <div className="p-6 border-b border-gray-200">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-900">Workflows</h1>
                        <p className="mt-2 text-gray-600">Visual workflow management and orchestration</p>
                    </div>
                    <div className="flex items-center space-x-3">
                        <button
                            onClick={refreshJobs}
                            className="inline-flex items-center px-3 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                        >
                            <RotateCcw className="w-4 h-4 mr-2"/>
                            Refresh
                        </button>
                    </div>
                </div>

                {/* View Mode Tabs */}
                <div className="mt-6">
                    <div className="border-b border-gray-200">
                        <nav className="-mb-px flex space-x-8">
                            {viewModes.map(({key, label, icon: Icon}) => (
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
                                    <Icon className="w-4 h-4 mr-2"/>
                                    {label}
                                </button>
                            ))}
                        </nav>
                    </div>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-hidden">
                {loading ? (
                    <div className="flex items-center justify-center h-full">
                        <div className="text-center">
                            <div
                                className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
                            <p className="text-gray-500">Loading workflow data...</p>
                        </div>
                    </div>
                ) : error ? (
                    <div className="flex items-center justify-center h-full">
                        <div className="text-center">
                            <p className="text-red-500 mb-4">Error: {error}</p>
                            <button
                                onClick={refreshJobs}
                                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
                            >
                                Retry
                            </button>
                        </div>
                    </div>
                ) : (
                    <>
                        {/* Graph View */}
                        {viewMode === 'graph' && (
                            <WorkflowGraph
                                jobs={workflowJobs}
                                onJobSelect={handleJobSelect}
                                onJobAction={handleJobAction}
                            />
                        )}

                        {/* Tree View */}
                        {viewMode === 'tree' && (
                            <div className="p-6">
                                <div className="bg-white rounded-lg shadow">
                                    <div className="p-6">
                                        <h3 className="text-lg font-medium text-gray-900 mb-4">Workflow Execution
                                            Tree</h3>
                                        {workflowJobs.length === 0 ? (
                                            <div className="text-center py-8">
                                                <List className="w-8 h-8 text-gray-400 mx-auto mb-2"/>
                                                <p className="text-gray-500">No workflow jobs found</p>
                                                <p className="text-sm text-gray-400 mt-1">Jobs with dependencies will
                                                    appear here</p>
                                            </div>
                                        ) : (
                                            <div className="space-y-4">
                                                {workflowJobs.map(job => (
                                                    <div key={job.id} className="border rounded-lg p-4">
                                                        <div className="flex items-center justify-between">
                                                            <div>
                                                                <h4 className="font-medium">{job.name || job.id.slice(0, 8)}</h4>
                                                                <p className="text-sm text-gray-500">{job.command}</p>
                                                                {job.dependsOn && job.dependsOn.length > 0 && (
                                                                    <p className="text-xs text-gray-400 mt-1">
                                                                        Depends on: {job.dependsOn.join(', ')}
                                                                    </p>
                                                                )}
                                                            </div>
                                                            <span className={clsx(
                                                                'px-2 py-1 rounded-full text-xs font-medium',
                                                                job.status === 'RUNNING' ? 'bg-yellow-100 text-yellow-800' :
                                                                    job.status === 'COMPLETED' ? 'bg-green-100 text-green-800' :
                                                                        job.status === 'FAILED' ? 'bg-red-100 text-red-800' :
                                                                            'bg-gray-100 text-gray-800'
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

                        {/* Gantt Chart View */}
                        {viewMode === 'gantt' && (
                            <div className="p-6">
                                <div className="bg-white rounded-lg shadow">
                                    <div className="p-6">
                                        <h3 className="text-lg font-medium text-gray-900 mb-4">Timeline View</h3>
                                        <div className="text-center py-8">
                                            <BarChart3 className="w-8 h-8 text-gray-400 mx-auto mb-2"/>
                                            <p className="text-gray-500">Timeline visualization coming soon</p>
                                            <p className="text-sm text-gray-400 mt-1">Gantt chart showing job execution
                                                timeline</p>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        )}
                    </>
                )}
            </div>

            {/* Status Bar */}
            <div className="border-t border-gray-200 px-6 py-3 bg-gray-50">
                <div className="flex items-center justify-between text-sm text-gray-600">
                    <div>
                        {workflowJobs.length > 0 ? (
                            <span>{workflowJobs.length} workflow jobs</span>
                        ) : (
                            <span>No workflow dependencies detected</span>
                        )}
                    </div>
                    {selectedJob && (
                        <div>
                            Selected: {selectedJob.name || selectedJob.id.slice(0, 8)} ({selectedJob.status})
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default Workflows;