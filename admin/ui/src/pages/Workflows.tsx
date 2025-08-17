import React, { useMemo, useState } from 'react';
import { useJobs } from '../hooks/useJobs';
import WorkflowList from '../components/Workflow/WorkflowList';
import WorkflowDetail from '../components/Workflow/WorkflowDetail';
import { Job, JobStatus } from '../types/job';
import { Plus, RotateCcw } from 'lucide-react';

type WorkflowStatus = 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';

interface Workflow {
    id: string;
    name: string;
    description?: string;
    jobs: Job[];
    status: WorkflowStatus;
    lastRun?: string;
    duration?: number;
}

const mapJobStatusToWorkflowStatus = (status: JobStatus): WorkflowStatus => {
    switch (status) {
        case 'INITIALIZING':
        case 'WAITING':
        case 'QUEUED':
            return 'QUEUED';
        case 'RUNNING':
            return 'RUNNING';
        case 'COMPLETED':
            return 'COMPLETED';
        case 'FAILED':
            return 'FAILED';
        case 'STOPPED':
            return 'STOPPED';
        default:
            return 'STOPPED';
    }
};

const Workflows: React.FC = () => {
    const { 
        jobs, 
        loading, 
        error, 
        refreshJobs
    } = useJobs();
    const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(null);

    // Group jobs into workflows based on dependencies
    const workflows = useMemo((): Workflow[] => {
        // Since most jobs don't have dependencies in this system,
        // we'll create example workflows from recent jobs
        if (jobs.length > 0) {
            const recentJobs = jobs.slice(0, Math.min(8, jobs.length));
            
            // Create mock workflows - only if we have enough jobs
            const workflow1Jobs = recentJobs.slice(0, Math.min(4, recentJobs.length));
            const workflow2Jobs = recentJobs.slice(4, Math.min(8, recentJobs.length));
            
            const workflow1 = workflow1Jobs.map((job, index) => ({
                ...job,
                dependsOn: index > 0 ? [workflow1Jobs[index - 1].id] : []
            }));
            
            const workflow2 = workflow2Jobs.map((job, index) => ({
                ...job,
                dependsOn: index > 0 ? [workflow2Jobs[index - 1].id] : []
            }));

            const workflows: Workflow[] = [];

            // Only add workflow 1 if we have jobs for it
            if (workflow1.length > 0) {
                workflows.push({
                    id: 'workflow-1',
                    name: 'Data Processing Pipeline',
                    description: 'Process and analyze data batch jobs',
                    jobs: workflow1,
                    status: mapJobStatusToWorkflowStatus(workflow1[workflow1.length - 1]?.status || 'COMPLETED'),
                    lastRun: workflow1[0]?.startTime || (workflow1[0] as any)?.start_time,
                    duration: workflow1.reduce((total, job) => {
                        const start = (job as any).start_time;
                        const end = (job as any).end_time;
                        if (start && end) {
                            return total + (new Date(end).getTime() - new Date(start).getTime());
                        }
                        return total;
                    }, 0)
                });
            }

            // Only add workflow 2 if we have jobs for it
            if (workflow2.length > 0) {
                workflows.push({
                    id: 'workflow-2', 
                    name: 'ML Training Pipeline',
                    description: 'Machine learning model training and evaluation',
                    jobs: workflow2,
                    status: mapJobStatusToWorkflowStatus(workflow2[workflow2.length - 1]?.status || 'COMPLETED'),
                    lastRun: workflow2[0]?.startTime || (workflow2[0] as any)?.start_time,
                    duration: workflow2.reduce((total, job) => {
                        const start = (job as any).start_time;
                        const end = (job as any).end_time;
                        if (start && end) {
                            return total + (new Date(end).getTime() - new Date(start).getTime());
                        }
                        return total;
                    }, 0)
                });
            }

            return workflows;
        }

        // Return empty array if no jobs
        return [];
    }, [jobs]);

    const selectedWorkflow = selectedWorkflowId 
        ? workflows.find(w => w.id === selectedWorkflowId)
        : null;

    const handleWorkflowClick = (workflowId: string) => {
        setSelectedWorkflowId(workflowId);
    };

    const handleBack = () => {
        setSelectedWorkflowId(null);
    };

    // Show workflow detail view if a workflow is selected
    if (selectedWorkflow) {
        return (
            <WorkflowDetail
                workflow={selectedWorkflow}
                onBack={handleBack}
                onRefresh={refreshJobs}
            />
        );
    }

    // Show workflow list view
    return (
        <div className="p-6">
            <div className="mb-8">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-3xl font-bold text-white">Workflows</h1>
                        <p className="mt-2 text-gray-300">Visual workflow management and orchestration</p>
                    </div>
                    <div className="flex space-x-3">
                        <button
                            onClick={refreshJobs}
                            className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                        >
                            <RotateCcw className="h-4 w-4 mr-2"/>
                            Refresh
                        </button>
                        <button
                            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
                            disabled
                        >
                            <Plus className="h-4 w-4 mr-2"/>
                            New Workflow
                        </button>
                    </div>
                </div>
            </div>

            {loading ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-white">Loading workflows...</p>
                    </div>
                </div>
            ) : error ? (
                <div className="bg-gray-800 rounded-lg shadow">
                    <div className="p-6">
                        <p className="text-red-500">Error: {error}</p>
                    </div>
                </div>
            ) : (
                <WorkflowList
                    workflows={workflows}
                    onWorkflowClick={handleWorkflowClick}
                    loading={loading}
                />
            )}
        </div>
    );
};

export default Workflows;