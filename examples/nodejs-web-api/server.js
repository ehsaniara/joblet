#!/usr/bin/env node

const express = require('express');
const bodyParser = require('body-parser');
const cors = require('cors');
const sqlite3 = require('sqlite3').verbose();
const path = require('path');
const fs = require('fs');

// Initialize Express app
const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(cors());
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Initialize SQLite database
const dbPath = path.join(__dirname, 'tasks.db');
const db = new sqlite3.Database(dbPath);

// Create tables if they don't exist
db.serialize(() => {
    db.run(`
        CREATE TABLE IF NOT EXISTS tasks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            type TEXT NOT NULL,
            status TEXT DEFAULT 'pending',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            completed_at DATETIME,
            result TEXT
        )
    `);

    db.run(`
        CREATE TABLE IF NOT EXISTS stats (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            total_tasks INTEGER DEFAULT 0,
            completed_tasks INTEGER DEFAULT 0,
            failed_tasks INTEGER DEFAULT 0,
            processing_time_ms INTEGER DEFAULT 0,
            last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `);

    // Initialize stats if not exists
    db.run(`INSERT OR IGNORE INTO stats (id) VALUES (1)`);
});

// Health check endpoint
app.get('/health', (req, res) => {
    res.json({
        status: 'healthy',
        service: 'joblet-nodejs-api',
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        memory: process.memoryUsage(),
        jobId: process.env.JOB_ID || 'unknown'
    });
});

// Get all tasks
app.get('/api/tasks', (req, res) => {
    const status = req.query.status;
    let query = 'SELECT * FROM tasks';
    const params = [];

    if (status) {
        query += ' WHERE status = ?';
        params.push(status);
    }

    query += ' ORDER BY created_at DESC';

    db.all(query, params, (err, rows) => {
        if (err) {
            return res.status(500).json({ error: err.message });
        }
        res.json({ tasks: rows, count: rows.length });
    });
});

// Create a new task
app.post('/api/tasks', (req, res) => {
    const { title, type } = req.body;

    if (!title || !type) {
        return res.status(400).json({ error: 'Title and type are required' });
    }

    const query = 'INSERT INTO tasks (title, type) VALUES (?, ?)';
    
    db.run(query, [title, type], function(err) {
        if (err) {
            return res.status(500).json({ error: err.message });
        }

        // Update stats
        db.run('UPDATE stats SET total_tasks = total_tasks + 1 WHERE id = 1');

        res.status(201).json({
            id: this.lastID,
            title,
            type,
            status: 'pending',
            created_at: new Date().toISOString()
        });
    });
});

// Get task by ID
app.get('/api/tasks/:id', (req, res) => {
    const { id } = req.params;

    db.get('SELECT * FROM tasks WHERE id = ?', [id], (err, row) => {
        if (err) {
            return res.status(500).json({ error: err.message });
        }
        if (!row) {
            return res.status(404).json({ error: 'Task not found' });
        }
        res.json(row);
    });
});

// Update task status
app.put('/api/tasks/:id', (req, res) => {
    const { id } = req.params;
    const { status, result } = req.body;

    if (!status) {
        return res.status(400).json({ error: 'Status is required' });
    }

    const validStatuses = ['pending', 'processing', 'completed', 'failed'];
    if (!validStatuses.includes(status)) {
        return res.status(400).json({ error: 'Invalid status' });
    }

    let query = 'UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP';
    const params = [status];

    if (status === 'completed' || status === 'failed') {
        query += ', completed_at = CURRENT_TIMESTAMP';
    }

    if (result) {
        query += ', result = ?';
        params.push(result);
    }

    query += ' WHERE id = ?';
    params.push(id);

    db.run(query, params, function(err) {
        if (err) {
            return res.status(500).json({ error: err.message });
        }
        if (this.changes === 0) {
            return res.status(404).json({ error: 'Task not found' });
        }

        // Update stats
        if (status === 'completed') {
            db.run('UPDATE stats SET completed_tasks = completed_tasks + 1 WHERE id = 1');
        } else if (status === 'failed') {
            db.run('UPDATE stats SET failed_tasks = failed_tasks + 1 WHERE id = 1');
        }

        res.json({ message: 'Task updated successfully' });
    });
});

// Delete a task
app.delete('/api/tasks/:id', (req, res) => {
    const { id } = req.params;

    db.run('DELETE FROM tasks WHERE id = ?', [id], function(err) {
        if (err) {
            return res.status(500).json({ error: err.message });
        }
        if (this.changes === 0) {
            return res.status(404).json({ error: 'Task not found' });
        }
        res.json({ message: 'Task deleted successfully' });
    });
});

// Get statistics
app.get('/api/stats', (req, res) => {
    db.get('SELECT * FROM stats WHERE id = 1', (err, stats) => {
        if (err) {
            return res.status(500).json({ error: err.message });
        }

        db.get('SELECT COUNT(*) as pending FROM tasks WHERE status = "pending"', (err, pending) => {
            if (err) {
                return res.status(500).json({ error: err.message });
            }

            db.get('SELECT COUNT(*) as processing FROM tasks WHERE status = "processing"', (err, processing) => {
                if (err) {
                    return res.status(500).json({ error: err.message });
                }

                res.json({
                    total_tasks: stats.total_tasks,
                    completed_tasks: stats.completed_tasks,
                    failed_tasks: stats.failed_tasks,
                    pending_tasks: pending.pending,
                    processing_tasks: processing.processing,
                    success_rate: stats.total_tasks > 0 
                        ? ((stats.completed_tasks / stats.total_tasks) * 100).toFixed(2) + '%'
                        : '0%',
                    average_processing_time: stats.completed_tasks > 0
                        ? Math.round(stats.processing_time_ms / stats.completed_tasks) + 'ms'
                        : '0ms',
                    last_updated: stats.last_updated
                });
            });
        });
    });
});

// 404 handler
app.use((req, res) => {
    res.status(404).json({ error: 'Endpoint not found' });
});

// Error handler
app.use((err, req, res, next) => {
    console.error('Error:', err);
    res.status(500).json({ error: 'Internal server error' });
});

// Graceful shutdown
const gracefulShutdown = () => {
    console.log('Received shutdown signal, closing server gracefully...');
    
    server.close(() => {
        console.log('HTTP server closed');
        
        db.close((err) => {
            if (err) {
                console.error('Error closing database:', err);
            } else {
                console.log('Database connection closed');
            }
            process.exit(0);
        });
    });

    // Force shutdown after 10 seconds
    setTimeout(() => {
        console.error('Forcing shutdown after timeout');
        process.exit(1);
    }, 10000);
};

process.on('SIGTERM', gracefulShutdown);
process.on('SIGINT', gracefulShutdown);

// Start server
const server = app.listen(PORT, '0.0.0.0', () => {
    console.log(`
============================================================
 JOBLET NODE.JS API SERVER
============================================================
Server started at: ${new Date().toISOString()}
Port: ${PORT}
Job ID: ${process.env.JOB_ID || 'unknown'}
Working directory: ${process.cwd()}
Node version: ${process.version}

API Endpoints:
  GET  /health           - Health check
  GET  /api/tasks        - List all tasks
  POST /api/tasks        - Create new task
  GET  /api/tasks/:id    - Get task by ID
  PUT  /api/tasks/:id    - Update task status
  DELETE /api/tasks/:id  - Delete task
  GET  /api/stats        - Get statistics

Server is ready to accept connections!
============================================================
    `);
});