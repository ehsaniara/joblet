import {useEffect, useRef} from 'react';
import {useSettings} from '../contexts/SettingsContext';

/**
 * Hook to automatically refresh data at user-defined intervals
 * @param callback Function to call for refreshing data
 * @param dependencies Array of dependencies that should trigger a refresh
 */
export const useAutoRefresh = (callback: () => void, dependencies: any[] = []) => {
    const {settings} = useSettings();
    const intervalRef = useRef<NodeJS.Timeout | null>(null);
    const callbackRef = useRef(callback);

    // Update callback ref when callback changes
    useEffect(() => {
        callbackRef.current = callback;
    }, [callback]);

    // Setup auto-refresh interval
    useEffect(() => {
        // Clear existing interval
        if (intervalRef.current) {
            clearInterval(intervalRef.current);
            intervalRef.current = null;
        }

        // Only set up interval if refresh is enabled
        if (settings.refreshFrequency > 0) {
            intervalRef.current = setInterval(() => {
                callbackRef.current();
            }, settings.refreshFrequency * 1000);
        }

        // Cleanup on unmount or settings change
        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
        };
    }, [settings.refreshFrequency, ...dependencies]);

    // Manual refresh function
    const refresh = () => {
        callbackRef.current();
    };

    return {refresh};
};