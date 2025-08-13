import {useCallback, useEffect, useState} from 'react';
import {Job, JobExecuteRequest} from '../types/job';
import {apiService} from '../services/apiService';

interface UseJobsReturn {
    jobs: Job[];
    loading: boolean;
    error: string | null;
    refreshJobs: () => Promise<void>;
    executeJob: (request: JobExecuteRequest) => Promise<string>;
    stopJob: (jobId: string) => Promise<void>;
}

export const useJobs = (): UseJobsReturn => {
    const [jobs, setJobs] = useState<Job[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    const refreshJobs = useCallback(async (): Promise<void> => {
        try {
            setError(null);
            const response = await apiService.getJobs();
            setJobs(response);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch jobs');
        } finally {
            setLoading(false);
        }
    }, []);

    const executeJob = useCallback(async (request: JobExecuteRequest): Promise<string> => {
        try {
            const response = await apiService.executeJob(request);
            await refreshJobs(); // Refresh job list
            return response.jobId;
        } catch (err) {
            throw new Error(err instanceof Error ? err.message : 'Failed to execute job');
        }
    }, [refreshJobs]);

    const stopJob = useCallback(async (jobId: string): Promise<void> => {
        try {
            await apiService.stopJob(jobId);
            await refreshJobs(); // Refresh job list
        } catch (err) {
            throw new Error(err instanceof Error ? err.message : 'Failed to stop job');
        }
    }, [refreshJobs]);

    useEffect(() => {
        refreshJobs();

        // Poll for updates every 2 seconds
        const interval = setInterval(refreshJobs, 2000);
        return () => clearInterval(interval);
    }, [refreshJobs]);

    return {jobs, loading, error, refreshJobs, executeJob, stopJob};
};