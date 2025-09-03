import {useCallback, useEffect, useRef, useState} from 'react';
import {Job, JobStatus, WorkflowJob} from '@/types';
import WorkflowMermaidGraph from './WorkflowMermaidGraph';
import WorkflowTimeline from './WorkflowTimeline';
import {BarChart3, Code, FileText, List, Network, X} from 'lucide-react';
import {apiService} from '@/services/apiService';
import {useLogStream} from '@/hooks/useLogStream.ts';
import clsx from 'clsx';

interface WorkflowDetailProps {
    workflowId: string;
    onRefresh: () => void; // Keep for compatibility but not used since WebSocket handles updates
}

interface WorkflowData {
    uuid: string;
    name: string;
    workflow: string;
    status: 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';
    total_jobs: number;
    completed_jobs: number;
    failed_jobs: number;
    created_at: string;
    started_at?: string;
    completed_at?: string;
    jobs: WorkflowJob[];
}

type ViewMode = 'graph' | 'tree' | 'timeline' | 'yaml';

const WorkflowDetail: React.FC<WorkflowDetailProps> = ({
                                                           workflowId,
                                                       }) => {
    const [viewMode, setViewMode] = useState<ViewMode>('graph');
    const [selectedJob, setSelectedJob] = useState<Job | null>(null);
    const [workflow, setWorkflow] = useState<WorkflowData | null>(null);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    // Job Details Modal State
    const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
    const [selectedRnxJobId, setSelectedRnxJobId] = useState<string | null>(null);
    const [activeTab, setActiveTab] = useState<'logs' | 'details'>('logs');
    const [selectedJobDetails, setSelectedJobDetails] = useState<Job | null>(null);
    const [jobLoading, setJobLoading] = useState<boolean>(false);
    const [autoScroll, setAutoScroll] = useState<boolean>(true);
    const logsContainerRef = useRef<HTMLDivElement>(null);

    // YAML content state
    const [yamlContent, setYamlContent] = useState<{
        content: string;
        filePath: string;
        lastModified: string;
        loading: boolean;
        error: string | null;
        note?: string;
    }>({
        content: '',
        filePath: '',
        lastModified: '',
        loading: false,
        error: null,
        note: undefined
    });
    // Use RNX job ID for log streaming if available, otherwise use UI job ID
    const {logs, connected, error: logError, clearLogs} = useLogStream(selectedRnxJobId || selectedJobId);

    const fetchWorkflow = useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const workflowData = await apiService.getWorkflow(workflowId);

            // Use real workflow jobs from the API
            // The server now returns properly formatted job data
            setWorkflow(workflowData);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch workflow');
        } finally {
            setLoading(false);
        }
    }, [workflowId]);

    useEffect(() => {
        void fetchWorkflow();
    }, [fetchWorkflow]);

    // Note: Auto-navigation removed - users can use escape key to go back

    // Auto-scroll to bottom when logs change
    useEffect(() => {
        if (autoScroll && logsContainerRef.current) {
            const container = logsContainerRef.current;
            container.scrollTop = container.scrollHeight;
        }
    }, [logs, autoScroll]);

    const handleJobSelect = (job: WorkflowJob | null) => {
        setSelectedJob(job);
        // Open job details dialog when job is selected from graph
        if (job) {
            void handleViewJob(job.id);
        }
    };

    const handleJobAction = (jobId: string, action: string) => {
        if (action === 'details') {
            void handleViewJob(jobId);
        }
    };

    const handleViewJob = async (jobId: string) => {
        setSelectedJobId(jobId);
        setJobLoading(true);

        // Clear previous RNX job ID state to start fresh
        setSelectedRnxJobId(null);

        // Find the workflow job info using UI job ID
        const workflowJob = workflow?.jobs.find(job => (job as any).id === jobId) as WorkflowJob | undefined;

        // Check if this job has actually started executing (not just queued/pending/cancelled)
        const hasStarted = workflowJob?.hasStarted;
        const rnxJobId = workflowJob?.rnxJobId;
        const jobStatus = workflowJob?.status;

        // Set RNX job ID for log streaming (only if it's a job that actually started executing)
        if (hasStarted && rnxJobId !== null && rnxJobId !== undefined && rnxJobId !== 0) {
            setSelectedRnxJobId(rnxJobId.toString());
        }

        if (!hasStarted) {
            // For jobs that haven't started executing, default to details tab since logs won't be available
            setActiveTab('details');
        } else {
            // For jobs that have started executing, default to logs tab
            setActiveTab('logs');
        }

        try {
            // Only try to fetch job details if the job has started executing and has a valid RNX job ID
            if (hasStarted && rnxJobId !== null && rnxJobId !== undefined && rnxJobId !== 0) {
                // Try to fetch real job details using the actual RNX job ID
                const jobDetails = await apiService.getJob(rnxJobId.toString());
                setSelectedJobDetails(jobDetails);
            } else {
                // For jobs that haven't started executing, create details from workflow info
                if (workflowJob) {
                    // Determine appropriate command name based on job status
                    let commandName;
                    switch (jobStatus?.toUpperCase()) {
                        case 'CANCELLED':
                            commandName = `${workflowJob.name || 'job'} (cancelled)`;
                            break;
                        case 'QUEUED':
                        case 'PENDING':
                            commandName = `${workflowJob.name || 'job'} (pending)`;
                            break;
                        default:
                            commandName = workflowJob.name || 'job';
                    }

                    setSelectedJobDetails({
                        id: jobId,
                        command: commandName,
                        args: [],
                        status: workflowJob.status as JobStatus,
                        startTime: '',
                        endTime: '',
                        duration: 0,
                        maxCPU: 0,
                        maxMemory: 0,
                        maxIOBPS: 0,
                        cpuCores: '',
                        runtime: '',
                        network: 'bridge',
                        volumes: [],
                        uploads: [],
                        uploadDirs: [],
                        envVars: {},
                        secretEnvVars: {},
                        dependsOn: workflowJob.dependsOn || [],
                        exitCode: undefined,
                        resourceUsage: undefined
                    });
                } else {
                    console.error('Job not found in workflow');
                }
            }
        } catch (error) {
            console.error('Failed to fetch job details:', error);

            // Fallback: use basic workflow job info
            if (workflowJob) {
                // Determine appropriate command name for fallback
                let fallbackCommand;
                if (!hasStarted) {
                    switch (jobStatus?.toUpperCase()) {
                        case 'CANCELLED':
                            fallbackCommand = `${workflowJob.name || 'job'} (cancelled)`;
                            break;
                        case 'QUEUED':
                        case 'PENDING':
                            fallbackCommand = `${workflowJob.name || 'job'} (pending)`;
                            break;
                        default:
                            fallbackCommand = workflowJob.name || 'job';
                    }
                } else {
                    fallbackCommand = 'workflow-job';
                }

                setSelectedJobDetails({
                    id: jobId,
                    command: fallbackCommand,
                    args: [],
                    status: workflowJob.status as JobStatus,
                    startTime: '',
                    endTime: '',
                    duration: 0,
                    maxCPU: 0,
                    maxMemory: 0,
                    maxIOBPS: 0,
                    cpuCores: '',
                    runtime: '',
                    network: 'bridge',
                    volumes: [],
                    uploads: [],
                    uploadDirs: [],
                    envVars: {},
                    secretEnvVars: {},
                    dependsOn: workflowJob.dependsOn || [],
                    exitCode: workflowJob.status === 'COMPLETED' ? 0 : undefined,
                    resourceUsage: undefined
                });
            } else {
                // Create error job details for display
                setSelectedJobDetails({
                    id: jobId,
                    command: 'unknown',
                    args: [],
                    status: 'ERROR' as JobStatus,
                    startTime: '',
                    endTime: '',
                    duration: 0,
                    maxCPU: 0,
                    maxMemory: 0,
                    maxIOBPS: 0,
                    cpuCores: '',
                    runtime: '',
                    network: 'bridge',
                    volumes: [],
                    uploads: [],
                    uploadDirs: [],
                    envVars: {},
                    secretEnvVars: {},
                    dependsOn: [],
                    exitCode: undefined,
                    resourceUsage: undefined
                });
            }
        } finally {
            setJobLoading(false);
        }
    };


    const handleCloseModal = () => {
        setSelectedJobId(null);
        setSelectedRnxJobId(null);
        setSelectedJobDetails(null);
        setActiveTab('logs');
        clearLogs();
    };

    // Handle escape key to close job details modal
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

    const getStatusColor = (status: string) => {
        switch (status) {
            case 'RUNNING':
                return 'bg-yellow-900 text-yellow-200';
            case 'COMPLETED':
                return 'bg-green-900 text-green-200';
            case 'FAILED':
                return 'bg-red-900 text-red-200';
            case 'STOPPED':
                return 'bg-gray-700 text-gray-200';
            case 'QUEUED':
                return 'bg-blue-900 text-blue-200';
            default:
                return 'bg-gray-700 text-gray-200';
        }
    };

    const fetchYamlContent = useCallback(async () => {
        try {
            setYamlContent(prev => ({...prev, loading: true, error: null}));
            const response = await fetch(`/api/workflows/${workflowId}/yaml`);

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || 'Failed to fetch YAML content');
            }

            const data = await response.json();
            setYamlContent({
                content: data.content,
                filePath: data.filePath,
                lastModified: data.lastModified,
                loading: false,
                error: null,
                note: data.note
            });
        } catch (error) {
            setYamlContent(prev => ({
                ...prev,
                loading: false,
                error: error instanceof Error ? error.message : 'Failed to fetch YAML content'
            }));
        }
    }, [workflowId]);

    // Fetch YAML content when switching to YAML view
    useEffect(() => {
        if (viewMode === 'yaml' && !yamlContent.content && !yamlContent.loading) {
            fetchYamlContent();
        }
    }, [viewMode, yamlContent.content, yamlContent.loading, fetchYamlContent]);

    const viewModes = [
        {key: 'graph' as ViewMode, label: 'Graph View', icon: Network},
        {key: 'tree' as ViewMode, label: 'Tree View', icon: List},
        {key: 'timeline' as ViewMode, label: 'Timeline', icon: BarChart3},
        {key: 'yaml' as ViewMode, label: 'YAML Source', icon: Code},
    ];

    if (loading) {
        return (
            <div className="flex flex-col h-full">
                <div className="p-6 border-b border-gray-200">
                    <div className="flex items-center space-x-4">
                        <div className="text-lg text-white">Loading workflow...</div>
                    </div>
                </div>
            </div>
        );
    }

    if (error || !workflow) {
        return (
            <div className="flex flex-col h-full">
                <div className="p-6 border-b border-gray-200">
                    <div className="flex items-center space-x-4">
                        <div className="text-lg text-red-500">Error: {error || 'Workflow not found'}</div>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="flex flex-col h-full">
            {/* Header */}
            <div className="p-6 border-b border-gray-200">
                <div className="flex items-center space-x-4 mb-4">
                    <div className="flex-1">
                        <div className="flex items-center space-x-3">
                            <h1 className="text-3xl font-bold text-white">{workflow.name}</h1>
                            <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(workflow.status)}`}
                            >
                                {workflow.status}
                            </span>
                        </div>
                        <p className="mt-2 text-gray-300">Workflow: {workflow.uuid}</p>
                    </div>
                    <div className="flex items-center text-sm">
                        <div className="w-2 h-2 rounded-full mr-2 bg-green-500 animate-pulse"></div>
                        <span className="text-gray-400">Live Updates</span>
                    </div>
                </div>

                {/* View Mode Tabs */}
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

            {/* Content */}
            <div className="flex-1 overflow-hidden">
                {/* Graph View */}
                {viewMode === 'graph' && (
                    <WorkflowMermaidGraph
                        jobs={workflow.jobs}
                        onJobSelect={handleJobSelect}
                        onJobAction={handleJobAction}
                    />
                )}

                {/* Tree View */}
                {viewMode === 'tree' && (
                    <div className="p-6">
                        <div className="bg-gray-800 rounded-lg shadow">
                            <div className="p-6">
                                <h3 className="text-lg font-medium text-white mb-4">
                                    Workflow Execution Tree
                                </h3>
                                {workflow.jobs.length === 0 ? (
                                    <div className="text-center py-8">
                                        <List className="w-8 h-8 text-gray-400 mx-auto mb-2"/>
                                        <p className="text-gray-500">No jobs executed in this workflow yet</p>
                                        <p className="text-sm text-gray-400 mt-2">Execute the workflow to see job
                                            details</p>
                                    </div>
                                ) : (
                                    <div className="space-y-4">
                                        {(() => {
                                            // Topological sort based on dependencies
                                            const jobs = [...workflow.jobs];
                                            const sorted: typeof jobs = [];
                                            const visited = new Set<string>();
                                            const temp = new Set<string>();

                                            // Helper function to find job by name or id
                                            const findJob = (nameOrId: string) =>
                                                jobs.find(j => j.name === nameOrId || j.id === nameOrId);

                                            // Depth-first search for topological sorting
                                            const visit = (job: typeof jobs[0]) => {
                                                if (temp.has(job.id)) return; // Circular dependency, skip
                                                if (visited.has(job.id)) return;

                                                temp.add(job.id);

                                                // Visit dependencies first
                                                if (job.dependsOn) {
                                                    for (const dep of job.dependsOn) {
                                                        const depJob = findJob(dep);
                                                        if (depJob) {
                                                            visit(depJob);
                                                        }
                                                    }
                                                }

                                                temp.delete(job.id);
                                                visited.add(job.id);
                                                sorted.push(job);
                                            };

                                            // Visit all jobs
                                            for (const job of jobs) {
                                                if (!visited.has(job.id)) {
                                                    visit(job);
                                                }
                                            }

                                            return sorted;
                                        })().map(job => (
                                            <div key={job.id}
                                                 className="border border-gray-600 rounded-lg p-4 hover:bg-gray-700 cursor-pointer"
                                                 onClick={() => {
                                                     void handleViewJob(job.id);
                                                 }}
                                                 style={{marginLeft: `${(job.dependsOn?.length || 0) * 20}px`}}>
                                                <div className="flex items-center justify-between">
                                                    <div className="flex-1">
                                                        <div className="flex items-center space-x-3">
                                                            {job.dependsOn && job.dependsOn.length > 0 && (
                                                                <span className="text-gray-500 text-xs">└─</span>
                                                            )}
                                                            <div>
                                                                <h4 className="font-medium text-white">{job.name || job.id}</h4>
                                                                <p className="text-xs text-gray-400">UUID: {job.id}</p>
                                                            </div>
                                                            <button
                                                                onClick={(e) => {
                                                                    e.stopPropagation();
                                                                    void handleViewJob(job.id);
                                                                }}
                                                                className="text-green-600 hover:text-green-300"
                                                                title="View Job Details & Logs"
                                                            >
                                                                <FileText className="h-4 w-4"/>
                                                            </button>
                                                        </div>
                                                        <p className="text-sm text-gray-300">
                                                            {job.command} {job.args?.join(' ') || ''}
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
                    <WorkflowTimeline
                        jobs={workflow.jobs}
                        onJobClick={handleViewJob}
                    />
                )}

                {/* YAML View */}
                {viewMode === 'yaml' && (
                    <div className="p-6">
                        {yamlContent.loading ? (
                            <div className="flex items-center justify-center py-12">
                                <div className="text-gray-600">Loading YAML content...</div>
                            </div>
                        ) : yamlContent.error ? (
                            <div className="bg-red-50 border border-red-200 rounded-md p-4">
                                <div className="flex items-center">
                                    <FileText className="h-5 w-5 text-red-400 mr-2"/>
                                    <h3 className="text-red-800 font-medium">Unable to load YAML content</h3>
                                </div>
                                <p className="text-red-700 mt-2">{yamlContent.error}</p>
                                <button
                                    onClick={fetchYamlContent}
                                    className="mt-3 inline-flex items-center px-3 py-2 border border-red-300 shadow-sm text-sm leading-4 font-medium rounded-md text-red-700 bg-white hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
                                >
                                    Retry
                                </button>
                            </div>
                        ) : (
                            <div>
                                <div className="mb-4 text-sm text-gray-600">
                                    <div><strong>Source:</strong> {yamlContent.note || 'Original workflow YAML'}</div>
                                    <div><strong>Last
                                        Modified:</strong> {new Date(yamlContent.lastModified).toLocaleString()}</div>
                                </div>
                                <div className="bg-gray-900 rounded-md p-4 overflow-auto max-h-[70vh] border">
                                    <pre className="text-gray-100 text-sm leading-relaxed whitespace-pre-wrap">
                                        <code>{yamlContent.content}</code>
                                    </pre>
                                </div>
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Status Bar */}
            <div className="border-t border-gray-800 px-6 py-3 bg-blue-950">
                <div className="flex items-center justify-between text-sm text-gray-300">
                    <div>
                        <span>{workflow.jobs.length} jobs in workflow</span>
                        {workflow.completed_at && (
                            <span className="ml-4">
                                Completed: {new Date(workflow.completed_at).toLocaleString()}
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

            {/* Job Details Modal */}
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
                                    Logs {workflow?.jobs.some(job => job.id === selectedJobId) &&
                                    <span className="text-xs">(Workflow)</span>}
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
                                                    {connected ? 'Connected' : 'Disconnected'}
                                                    {workflow?.jobs.some(job => job.id === selectedJobId) && ' (Workflow Job)'}
                                                </span>
                                            </div>
                                            {/* Show auto-scroll for jobs that have logs available (started jobs or non-workflow jobs) */}
                                            {(selectedRnxJobId || !workflow?.jobs.some(job => job.id === selectedJobId)) && (
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
                                            )}
                                        </div>
                                        <button
                                            onClick={clearLogs}
                                            className="px-3 py-1 text-sm bg-gray-500 hover:bg-gray-600 text-white rounded"
                                        >
                                            Clear Logs
                                        </button>
                                    </div>

                                    {logError && !workflow?.jobs.some(job => job.id === selectedJobId) && (
                                        <div
                                            className="mb-4 p-3 bg-red-100 dark:bg-red-900 border border-red-400 text-red-700 dark:text-red-300 rounded">
                                            Error: {logError}
                                        </div>
                                    )}

                                    <div
                                        ref={logsContainerRef}
                                        className="bg-black text-green-400 p-4 rounded-lg h-[70vh] overflow-y-auto font-mono text-sm"
                                    >
                                        {/* Display real logs for started jobs, appropriate message for other job states */}
                                        {!selectedRnxJobId ? (
                                            <div className="text-gray-500">
                                                {(() => {
                                                    const job = workflow?.jobs.find(j => j.id === selectedJobId);
                                                    const status = job?.status?.toUpperCase();
                                                    if (status === 'CANCELLED') return 'Job was cancelled. No logs available.';
                                                    if (status === 'QUEUED' || status === 'PENDING') return 'Job is queued/pending. No logs yet.';
                                                    return 'Job has not started executing. No logs available.';
                                                })()}
                                            </div>
                                        ) : logs.length === 0 ? (
                                            <div className="text-gray-500">
                                                {workflow?.jobs.some(job => job.id === selectedJobId)
                                                    ? "Loading workflow job logs..."
                                                    : "No logs available yet..."
                                                }
                                            </div>
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
                                            <span className="ml-3 text-gray-600 dark:text-gray-400">Loading job details...</span>
                                        </div>
                                    ) : selectedJobDetails ? (
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
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">{selectedJobDetails.id}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Status</dt>
                                                        <dd className="mt-1">
                                                            <span
                                                                className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${getStatusColor(selectedJobDetails.status)}`}>
                                                                {selectedJobDetails.status}
                                                            </span>
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Command</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">{selectedJobDetails.command}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Arguments</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white font-mono">
                                                            {selectedJobDetails.args?.length ? selectedJobDetails.args.join(' ') : 'None'}
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Duration</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{formatDuration(selectedJobDetails.duration)}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Exit
                                                            Code
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.exitCode ?? 'N/A'}</dd>
                                                    </div>
                                                </dl>
                                            </div>

                                            {/* Resource Limits */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Resource
                                                    Limits</h4>
                                                <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            CPU (%)
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.maxCPU || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            Memory (MB)
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.maxMemory || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Max
                                                            IO BPS
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.maxIOBPS || 'Unlimited'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">CPU
                                                            Cores
                                                        </dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.cpuCores || 'Default'}</dd>
                                                    </div>
                                                </dl>
                                            </div>

                                            {/* Configuration */}
                                            <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Configuration</h4>
                                                <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Runtime</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.runtime || 'Default'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Network</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">{selectedJobDetails.network || 'Default'}</dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Volumes</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                            {selectedJobDetails.volumes?.length ? selectedJobDetails.volumes.join(', ') : 'None'}
                                                        </dd>
                                                    </div>
                                                    <div>
                                                        <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Uploads</dt>
                                                        <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                                                            {selectedJobDetails.uploads?.length ? selectedJobDetails.uploads.join(', ') : 'None'}
                                                        </dd>
                                                    </div>
                                                </dl>
                                            </div>

                                            {/* Dependencies */}
                                            {selectedJobDetails.dependsOn && selectedJobDetails.dependsOn.length > 0 && (
                                                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                    <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Dependencies</h4>
                                                    <div className="space-y-2">
                                                        {selectedJobDetails.dependsOn.map((dependency, index) => {
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
                                            {(Object.keys(selectedJobDetails.envVars || {}).length > 0 || Object.keys(selectedJobDetails.secretEnvVars || {}).length > 0) && (
                                                <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
                                                    <h4 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Environment
                                                        Variables</h4>

                                                    {Object.keys(selectedJobDetails.envVars || {}).length > 0 && (
                                                        <div className="mb-4">
                                                            <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Regular
                                                                Variables</h5>
                                                            <div className="space-y-1">
                                                                {Object.entries(selectedJobDetails.envVars || {}).map(([key, value]) => (
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

                                                    {Object.keys(selectedJobDetails.secretEnvVars || {}).length > 0 && (
                                                        <div>
                                                            <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Secret
                                                                Variables</h5>
                                                            <div className="space-y-1">
                                                                {Object.keys(selectedJobDetails.secretEnvVars || {}).map((key) => (
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
        </div>
    );
};

export default WorkflowDetail;