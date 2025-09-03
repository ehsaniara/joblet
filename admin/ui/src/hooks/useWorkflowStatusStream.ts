import {useCallback, useEffect, useRef, useState} from 'react';
import {useNode} from '../contexts/NodeContext';
import {WorkflowJob} from '../types/job';

interface UseWorkflowStatusStreamReturn {
    connected: boolean;
    error: string | null;
    lastUpdate: number;
}

export const useWorkflowStatusStream = (
    workflowId: string | null,
    onJobStatusUpdate: (jobId: string, status: string, updatedJob?: WorkflowJob) => void
): UseWorkflowStatusStreamReturn => {
    const [connected, setConnected] = useState<boolean>(false);
    const [error, setError] = useState<string | null>(null);
    const [lastUpdate, setLastUpdate] = useState<number>(0);
    const wsRef = useRef<WebSocket | null>(null);
    const {selectedNode} = useNode();
    const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const reconnectAttemptsRef = useRef<number>(0);
    const maxReconnectAttempts = 5;
    const reconnectDelay = 3000; // 3 seconds

    const clearReconnectTimeout = useCallback(() => {
        if (reconnectTimeoutRef.current) {
            clearTimeout(reconnectTimeoutRef.current);
            reconnectTimeoutRef.current = null;
        }
    }, []);

    const connectWebSocket = useCallback(() => {
        if (!workflowId) {
            setConnected(false);
            return;
        }

        // Close existing connection
        if (wsRef.current) {
            wsRef.current.close();
        }

        try {
            const wsUrl = `ws://${window.location.host}/ws/workflow-status/${workflowId}?node=${encodeURIComponent(selectedNode)}`;
            const ws = new WebSocket(wsUrl);
            wsRef.current = ws;

            ws.onopen = () => {
                console.log(`Workflow status WebSocket connected for workflow: ${workflowId}`);
                setConnected(true);
                setError(null);
                reconnectAttemptsRef.current = 0;
            };

            ws.onmessage = (event: MessageEvent) => {
                try {
                    const update = JSON.parse(event.data);
                    setLastUpdate(Date.now());

                    if (update.type === 'job_status_change') {
                        // Handle individual job status updates
                        onJobStatusUpdate(update.jobId, update.status, update.job);
                    } else if (update.type === 'workflow_status_change') {
                        // Handle workflow-level status updates
                        if (update.jobs && Array.isArray(update.jobs)) {
                            // Batch update all jobs
                            update.jobs.forEach((job: WorkflowJob) => {
                                onJobStatusUpdate(job.id, job.status, job);
                            });
                        }
                    } else if (update.type === 'heartbeat') {
                        // Keep connection alive
                        setLastUpdate(Date.now());
                    }
                } catch (parseError) {
                    console.warn('Failed to parse workflow status update:', parseError);
                }
            };

            ws.onerror = (wsError) => {
                console.error('Workflow status WebSocket error:', wsError);
                setError('WebSocket connection error');
            };

            ws.onclose = (event) => {
                console.log('Workflow status WebSocket closed:', event.code, event.reason);
                setConnected(false);

                // Attempt to reconnect if not intentionally closed
                if (event.code !== 1000 && reconnectAttemptsRef.current < maxReconnectAttempts) {
                    reconnectAttemptsRef.current++;
                    console.log(`Attempting to reconnect workflow status WebSocket (${reconnectAttemptsRef.current}/${maxReconnectAttempts})`);

                    reconnectTimeoutRef.current = setTimeout(() => {
                        connectWebSocket();
                    }, reconnectDelay);
                } else if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
                    setError('Connection lost. Please refresh the page to reconnect.');
                }
            };

        } catch (connectionError) {
            console.error('Failed to create workflow status WebSocket:', connectionError);
            setError('Failed to establish WebSocket connection');
        }
    }, [workflowId, selectedNode, onJobStatusUpdate]);

    useEffect(() => {
        connectWebSocket();

        return () => {
            clearReconnectTimeout();
            if (wsRef.current) {
                wsRef.current.close(1000, 'Component unmounting');
                wsRef.current = null;
            }
        };
    }, [connectWebSocket, clearReconnectTimeout]);

    return {connected, error, lastUpdate};
};