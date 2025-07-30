#!/usr/bin/env node

const sqlite3 = require('sqlite3').verbose();
const path = require('path');

// Configuration
const POLL_INTERVAL = 5000; // 5 seconds
const WORKER_ID = process.env.WORKER_ID || `worker-${process.pid}`;

// Initialize database connection
const dbPath = path.join(__dirname, 'tasks.db');
const db = new sqlite3.Database(dbPath);

console.log(`
============================================================
 JOBLET BACKGROUND WORKER
============================================================
Worker started at: ${new Date().toISOString()}
Worker ID: ${WORKER_ID}
Job ID: ${process.env.JOB_ID || 'unknown'}
Poll interval: ${POLL_INTERVAL}ms
Database: ${dbPath}

Worker is processing tasks...
============================================================
`);

// Process a single task
async function processTask(task) {
    const startTime = Date.now();
    console.log(`[${WORKER_ID}] Processing task ${task.id}: ${task.title} (${task.type})`);

    try {
        // Mark task as processing
        await runQuery(
            'UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?',
            ['processing', task.id]
        );

        // Simulate different task types with different processing times
        let result;
        switch (task.type) {
            case 'data-processing':
                await simulateDataProcessing();
                result = 'Data processed successfully: 1000 records analyzed';
                break;

            case 'email-notification':
                await simulateEmailSending();
                result = 'Email sent to 50 recipients';
                break;

            case 'report-generation':
                await simulateReportGeneration();
                result = 'Report generated: report_' + Date.now() + '.pdf';
                break;

            case 'api-sync':
                await simulateApiSync();
                result = 'API sync completed: 200 records synchronized';
                break;

            default:
                await simulateGenericTask();
                result = 'Task completed successfully';
        }

        // Mark task as completed
        const processingTime = Date.now() - startTime;
        await runQuery(
            'UPDATE tasks SET status = ?, result = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?',
            ['completed', result, task.id]
        );

        // Update statistics
        await runQuery(
            'UPDATE stats SET completed_tasks = completed_tasks + 1, processing_time_ms = processing_time_ms + ? WHERE id = 1',
            [processingTime]
        );

        console.log(`[${WORKER_ID}] ✅ Task ${task.id} completed in ${processingTime}ms: ${result}`);

    } catch (error) {
        console.error(`[${WORKER_ID}] ❌ Task ${task.id} failed:`, error.message);

        // Mark task as failed
        await runQuery(
            'UPDATE tasks SET status = ?, result = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?',
            ['failed', error.message, task.id]
        );

        // Update statistics
        await runQuery(
            'UPDATE stats SET failed_tasks = failed_tasks + 1 WHERE id = 1'
        );
    }
}

// Simulation functions for different task types
async function simulateDataProcessing() {
    await sleep(2000 + Math.random() * 3000); // 2-5 seconds
}

async function simulateEmailSending() {
    await sleep(1000 + Math.random() * 2000); // 1-3 seconds
}

async function simulateReportGeneration() {
    await sleep(3000 + Math.random() * 4000); // 3-7 seconds
}

async function simulateApiSync() {
    await sleep(2000 + Math.random() * 2000); // 2-4 seconds
}

async function simulateGenericTask() {
    await sleep(1000 + Math.random() * 2000); // 1-3 seconds
}

// Helper function to run database queries
function runQuery(query, params = []) {
    return new Promise((resolve, reject) => {
        db.run(query, params, function(err) {
            if (err) reject(err);
            else resolve({ lastID: this.lastID, changes: this.changes });
        });
    });
}

// Helper function to get pending tasks
function getPendingTasks() {
    return new Promise((resolve, reject) => {
        db.all(
            'SELECT * FROM tasks WHERE status = ? ORDER BY created_at ASC LIMIT 5',
            ['pending'],
            (err, rows) => {
                if (err) reject(err);
                else resolve(rows);
            }
        );
    });
}

// Sleep helper
function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// Main worker loop
async function workerLoop() {
    while (true) {
        try {
            const tasks = await getPendingTasks();
            
            if (tasks.length > 0) {
                console.log(`[${WORKER_ID}] Found ${tasks.length} pending tasks`);
                
                // Process tasks sequentially
                for (const task of tasks) {
                    await processTask(task);
                }
            } else {
                console.log(`[${WORKER_ID}] No pending tasks, waiting...`);
            }

            // Wait before next poll
            await sleep(POLL_INTERVAL);

        } catch (error) {
            console.error(`[${WORKER_ID}] Worker error:`, error);
            await sleep(POLL_INTERVAL);
        }
    }
}

// Graceful shutdown
const gracefulShutdown = () => {
    console.log(`[${WORKER_ID}] Received shutdown signal, closing worker gracefully...`);
    
    db.close((err) => {
        if (err) {
            console.error('Error closing database:', err);
        } else {
            console.log('Database connection closed');
        }
        process.exit(0);
    });
};

process.on('SIGTERM', gracefulShutdown);
process.on('SIGINT', gracefulShutdown);

// Handle uncaught errors
process.on('uncaughtException', (error) => {
    console.error(`[${WORKER_ID}] Uncaught exception:`, error);
    gracefulShutdown();
});

process.on('unhandledRejection', (reason, promise) => {
    console.error(`[${WORKER_ID}] Unhandled rejection at:`, promise, 'reason:', reason);
    gracefulShutdown();
});

// Start the worker
workerLoop().catch(error => {
    console.error(`[${WORKER_ID}] Fatal error:`, error);
    process.exit(1);
});