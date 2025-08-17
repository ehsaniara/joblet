import {useCallback, useEffect, useState} from 'react';
import {DetailedSystemInfo} from '../types/monitor';
import {apiService} from '../services/apiService';

interface UseSystemInfoReturn {
    systemInfo: DetailedSystemInfo | null;
    loading: boolean;
    error: string | null;
    refetch: () => void;
}

export const useSystemInfo = (): UseSystemInfoReturn => {
    const [systemInfo, setSystemInfo] = useState<DetailedSystemInfo | null>(null);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);

    const fetchSystemInfo = useCallback(async (): Promise<void> => {
        try {
            setLoading(true);
            setError(null);
            const response = await apiService.getDetailedSystemInfo();
            setSystemInfo(response);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to fetch system information');
        } finally {
            setLoading(false);
        }
    }, []);

    const refetch = useCallback(() => {
        fetchSystemInfo();
    }, [fetchSystemInfo]);

    useEffect(() => {
        fetchSystemInfo();
    }, [fetchSystemInfo]);

    return {systemInfo, loading, error, refetch};
};