// Node.js version check - ensure we have Node.js 18+
const nodeVersion = process.version;
const majorVersion = parseInt(nodeVersion.slice(1).split('.')[0]);

if (majorVersion < 18) {
    console.error(`❌ Node.js ${nodeVersion} detected, but Node.js 18+ is required`);
    console.error(`💡 Please upgrade Node.js to version 18 or later`);
    console.error(`   Visit: https://nodejs.org/`);
    process.exit(1);
}

import express from 'express';
import {createServer} from 'http';
import {config} from './config.js';
import {setupFallbackRoutes, setupMiddleware} from './middleware/index.js';
import {setupRoutes} from './routes/index.js';
import {setupWebSocket} from './websocket/index.js';

// Create Express app and HTTP server
const app = express();
const server = createServer(app);

// Setup middleware
setupMiddleware(app);

// Setup API routes
setupRoutes(app);

// Setup fallback routes (must be after API routes)
setupFallbackRoutes(app);

// Setup WebSocket handling
const wss = setupWebSocket(server);

// Start server
server.listen(config.PORT, config.BIND_ADDRESS, () => {
    console.log(`🚀 Joblet Admin Server running at http://${config.BIND_ADDRESS}:${config.PORT}`);
    console.log(`📡 API endpoints available at /api/*`);
    console.log(`🔌 WebSocket endpoints available at /ws/*`);
    console.log(`🎨 Admin UI served from /`);
});

// Graceful shutdown
process.on('SIGINT', () => {
    console.log('🛑 Shutting down server...');
    server.close(() => {
        console.log('✅ Server closed');
        process.exit(0);
    });
});

export {app, server, wss};