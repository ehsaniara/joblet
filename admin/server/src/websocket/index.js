import {WebSocketServer} from 'ws';
import {
    handleJobMetricsStream,
    handleLogStream,
    handleMonitorStream,
    handleRuntimeInstallStream,
    handleWorkflowStatusStream
} from './handlers.js';

export function setupWebSocket(server) {
    const wss = new WebSocketServer({server});

    wss.on('connection', (ws, request) => {
        const url = new URL(request.url, `http://${request.headers.host}`);
        const pathname = url.pathname;
        const searchParams = url.searchParams;

        if (pathname.startsWith('/ws/logs/')) {
            // Log streaming
            const jobId = pathname.replace('/ws/logs/', '');
            handleLogStream(ws, jobId);
        } else if (pathname.startsWith('/ws/workflow-status/')) {
            // Workflow status streaming
            const workflowId = pathname.replace('/ws/workflow-status/', '');
            const node = searchParams.get('node');
            handleWorkflowStatusStream(ws, workflowId, node);
        } else if (pathname === '/ws/monitor') {
            // Monitor streaming
            const node = searchParams.get('node');
            handleMonitorStream(ws, node);
        } else if (pathname.startsWith('/ws/runtime-install/')) {
            // Runtime installation streaming
            const buildJobId = pathname.replace('/ws/runtime-install/', '');
            const node = searchParams.get('node');
            handleRuntimeInstallStream(ws, buildJobId, node);
        } else if (pathname.startsWith('/ws/metrics/')) {
            // Job metrics streaming
            const jobId = pathname.replace('/ws/metrics/', '');
            handleJobMetricsStream(ws, jobId);
        } else {
            ws.close(1000, 'Unknown WebSocket endpoint');
        }
    });

    return wss;
}