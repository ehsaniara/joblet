import {useCallback, useEffect, useState} from 'react';
import {apiService} from '../services/apiService';

type WorkflowStatus = 'RUNNING' | 'COMPLETED' | 'FAILED' | 'QUEUED' | 'STOPPED';

export interface Workflow {
    id: string | number; // Support both UUID strings and legacy numeric IDs
    name: string;
    workflow: string;
    status: WorkflowStatus;
    total_jobs: number;
    completed_jobs: number;
    failed_jobs: number;
    created_at: string;
    started_at?: string;
    completed_at?: string;
}

interface UseWorkflowsReturn {
    workflows: Workflow[];
    loading: boolean;
    error: string | null;
    refreshWorkflows: () => Promise<void>;
    // Pagination
    currentPage: number;
    pageSize: number;
    totalWorkflows: number;
    totalPages: number;
    paginatedWorkflows: Workflow[];
    setCurrentPage: (page: number) => void;
    setPageSize: (size: number) => void;
}

export const useWorkflows = (): UseWorkflowsReturn => {
    const [workflows, setWorkflows] = useState<Workflow[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    // Pagination state
    const [currentPage, setCurrentPage] = useState<number>(1);
    const [pageSize, setPageSize] = useState<number>(10);

    const fetchWorkflows = useCallback(async (showLoading = false): Promise<void> => {
        try {
            if (showLoading) {
                setLoading(true);
            }
            setError(null);
            const response = await apiService.getWorkflows();

            // Transform API response: map 'uuid' to 'id' and add name field
            const transformedWorkflows = response.map(workflow => ({
                ...workflow,
                id: workflow.uuid || workflow.id, // Map uuid to id for consistency
                name: workflow.workflow || `Workflow ${workflow.uuid ? workflow.uuid.substring(0, 8) : workflow.id}` // Use workflow filename as name
            }));

            // Sort workflows by creation date (newest first), fallback to ID comparison
            const sortedWorkflows = transformedWorkflows.sort((a, b) => {
                // First try to sort by created_at timestamp
                if (a.created_at && b.created_at) {
                    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
                }
                // Fallback to ID comparison only if both are numbers
                if (typeof a.id === 'number' && typeof b.id === 'number') {
                    return b.id - a.id;
                }
                // For UUIDs or mixed types, use string comparison
                return String(b.id).localeCompare(String(a.id));
            });
            setWorkflows(sortedWorkflows);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch workflows');
            console.error('Failed to fetch workflows:', err);
        } finally {
            if (showLoading) {
                setLoading(false);
            }
        }
    }, []);

    const refreshWorkflows = useCallback(async (): Promise<void> => {
        await fetchWorkflows(true);
    }, [fetchWorkflows]);

    // Calculate pagination values
    const totalWorkflows = workflows.length;
    const totalPages = Math.ceil(totalWorkflows / pageSize);
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedWorkflows = workflows.slice(startIndex, endIndex);

    // Reset to page 1 if current page is beyond available pages
    useEffect(() => {
        if (currentPage > totalPages && totalPages > 0) {
            setCurrentPage(1);
        }
    }, [currentPage, totalPages]);

    // Handle page size changes
    const handleSetPageSize = useCallback((size: number) => {
        setPageSize(size);
        setCurrentPage(1); // Reset to first page when changing page size
    }, []);

    useEffect(() => {
        // Initial load with loading indicator
        fetchWorkflows(true);

        // Poll for updates every 10 seconds (without loading indicator)
        const interval = setInterval(() => fetchWorkflows(false), 10000);
        return () => clearInterval(interval);
    }, [fetchWorkflows]);

    return {
        workflows,
        loading,
        error,
        refreshWorkflows,
        currentPage,
        pageSize,
        totalWorkflows,
        totalPages,
        paginatedWorkflows,
        setCurrentPage,
        setPageSize: handleSetPageSize
    };
};