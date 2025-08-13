import {useCallback, useEffect, useState} from 'react';
import {SystemMetrics} from '../types/monitor';
import {apiService} from '../services/apiService';

interface UseMonitorReturn {
    metrics: SystemMetrics | null;
    loading: boolean;
    error: string | null;
    isRealtime: boolean;
    toggleRealtime: () => void;
}

export const useMonitor = (): UseMonitorReturn => {
    const [metrics, setMetrics] = useState<SystemMetrics | null>(null);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);
    const [isRealtime, setIsRealtime] = useState<boolean>(false);

    const fetchMetrics = useCallback(async (): Promise<void> => {
        try {
            setError(null);
            const response = await apiService.getSystemMetrics();
            setMetrics(response);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch metrics');
        } finally {
            setLoading(false);
        }
    }, []);

    const toggleRealtime = useCallback(() => {
        setIsRealtime(prev => !prev);
    }, []);

    useEffect(() => {
        fetchMetrics();

        if (isRealtime) {
            const interval = setInterval(fetchMetrics, 5000);
            return () => clearInterval(interval);
        }
    }, [fetchMetrics, isRealtime]);

    return {metrics, loading, error, isRealtime, toggleRealtime};
};