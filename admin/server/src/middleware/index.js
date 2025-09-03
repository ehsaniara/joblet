import express from 'express';
import cors from 'cors';
import path from 'path';
import {fileURLToPath} from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export function setupMiddleware(app) {
    // Basic middleware
    app.use(cors());
    app.use(express.json());

    // Serve static files from React build
    app.use(express.static(path.join(__dirname, '../../../ui/dist')));
}

export function setupFallbackRoutes(app) {
    // Catch-all handler for SPA routing (must be called after API routes)
    app.get('*', (req, res) => {
        if (req.path.startsWith('/api/') || req.path.startsWith('/ws/')) {
            return res.status(404).json({error: 'API endpoint not found'});
        }
        res.sendFile(path.join(__dirname, '../../../ui/dist/index.html'));
    });
}