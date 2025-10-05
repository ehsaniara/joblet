import {useCallback, useEffect, useRef, useState} from 'react';

interface MetricPoint {
    jobId: string;
    timestamp: number;
    sampleIntervalSeconds: number;
    cpu: {
        usage?: number;
        [key: string]: any;
    };
    memory: {
        current?: number;
        limit?: number;
        [key: string]: any;
    };
    io: {
        readBytes?: number;
        writeBytes?: number;
        [key: string]: any;
    };
    process: {
        [key: string]: any;
    };
    cgroupPath?: string;
    limits: {
        [key: string]: any;
    };
    [key: string]: any;
}

interface UseMetricsStreamReturn {
    metrics: MetricPoint[];
    connected: boolean;
    error: string | null;
    clearMetrics: () => void;
}

export const useMetricsStream = (jobId: string | null): UseMetricsStreamReturn => {
    const [metrics, setMetrics] = useState<MetricPoint[]>([]);
    const [connected, setConnected] = useState<boolean>(false);
    const [error, setError] = useState<string | null>(null);
    const wsRef = useRef<WebSocket | null>(null);

    const clearMetrics = useCallback(() => {
        setMetrics([]);
    }, []);

    useEffect(() => {
        if (!jobId) {
            setConnected(false);
            setMetrics([]);
            return;
        }

        const wsUrl = `ws://localhost:5174/ws/metrics/${jobId}`;
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            setConnected(true);
            setError(null);
        };

        ws.onmessage = (event: MessageEvent) => {
            try {
                const message = JSON.parse(event.data);

                if (message.type === 'metrics' && message.data) {
                    // Single metric point from live stream
                    setMetrics(prev => [...prev, message.data]);
                } else if (message.type === 'error') {
                    setError(message.message);
                } else if (message.type === 'connection') {
                    // Connection status messages - can be logged if needed
                    console.log('Metrics stream:', message.message);
                }
            } catch (err) {
                console.error('Failed to parse metrics message:', err);
            }
        };

        ws.onerror = () => {
            setError('WebSocket connection error');
            setConnected(false);
        };

        ws.onclose = () => {
            setConnected(false);
        };

        return () => {
            if (wsRef.current) {
                wsRef.current.close();
                wsRef.current = null;
            }
        };
    }, [jobId]);

    return {
        metrics,
        connected,
        error,
        clearMetrics
    };
};