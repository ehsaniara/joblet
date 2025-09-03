import {useCallback, useEffect, useRef, useState} from 'react';
import {useNode} from '../contexts/NodeContext';
import {SystemMetrics} from '../types/monitor';

interface UseMonitorStreamReturn {
    metrics: SystemMetrics | null;
    connected: boolean;
    error: string | null;
    isRealtime: boolean;
    toggleRealtime: () => void;
}

export const useMonitorStream = (): UseMonitorStreamReturn => {
    const [metrics, setMetrics] = useState<SystemMetrics | null>(null);
    const [connected, setConnected] = useState<boolean>(false);
    const [error, setError] = useState<string | null>(null);
    const [isRealtime] = useState<boolean>(true); // Always enabled
    const wsRef = useRef<WebSocket | null>(null);
    const {selectedNode} = useNode();

    const toggleRealtime = useCallback(() => {
        // No-op since realtime is always on
    }, []);

    useEffect(() => {
        // Always create WebSocket connection
        const wsUrl = `ws://${window.location.host}/ws/monitor?node=${encodeURIComponent(selectedNode)}`;
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            setConnected(true);
            setError(null);
        };

        ws.onmessage = (event: MessageEvent) => {
            try {
                const message = JSON.parse(event.data);

                if (message.type === 'metrics') {
                    setMetrics(message.data);
                } else if (message.type === 'error') {
                    setError(message.message);
                }
            } catch (err) {
                setError('Failed to parse WebSocket message');
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
    }, [selectedNode]); // Removed isRealtime dependency since it's always true

    return {metrics, connected, error, isRealtime, toggleRealtime};
};