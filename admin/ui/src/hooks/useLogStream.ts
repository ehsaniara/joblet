import {useCallback, useEffect, useRef, useState} from 'react';

interface UseLogStreamReturn {
    logs: string[];
    connected: boolean;
    error: string | null;
    clearLogs: () => void;
}

export const useLogStream = (jobId: string | null): UseLogStreamReturn => {
    const [logs, setLogs] = useState<string[]>([]);
    const [connected, setConnected] = useState<boolean>(false);
    const [error, setError] = useState<string | null>(null);
    const wsRef = useRef<WebSocket | null>(null);

    const clearLogs = useCallback(() => {
        setLogs([]);
    }, []);

    useEffect(() => {
        if (!jobId) {
            setConnected(false);
            setLogs([]);
            return;
        }

        const wsUrl = `ws://${window.location.host}/ws/logs/${jobId}`;
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            setConnected(true);
            setError(null);
        };

        ws.onmessage = (event: MessageEvent) => {
            try {
                const logEntry = JSON.parse(event.data);
                const timestamp = new Date().toLocaleTimeString();
                
                if (logEntry.type === 'log') {
                    setLogs(prev => [...prev, `[${timestamp}] ${logEntry.message}`]);
                } else if (logEntry.type === 'error') {
                    setLogs(prev => [...prev, `[${timestamp}] ERROR: ${logEntry.message}`]);
                } else if (logEntry.type === 'connection') {
                    setLogs(prev => [...prev, `[${timestamp}] ${logEntry.message}`]);
                } else if (logEntry.type === 'status') {
                    setLogs(prev => [...prev, `[${timestamp}] STATUS: ${logEntry.message}`]);
                } else {
                    // Fallback for unknown message types
                    setLogs(prev => [...prev, `[${timestamp}] ${logEntry.message || JSON.stringify(logEntry)}`]);
                }
            } catch {
                // Fallback for plain text logs
                const timestamp = new Date().toLocaleTimeString();
                setLogs(prev => [...prev, `[${timestamp}] ${event.data}`]);
            }
        };

        ws.onerror = () => {
            setError('WebSocket connection error');
        };

        ws.onclose = () => {
            setConnected(false);
        };

        return () => {
            ws.close();
            wsRef.current = null;
        };
    }, [jobId]);

    return {logs, connected, error, clearLogs};
};