# Node.js Web API Example

This example demonstrates how to run a Node.js web API server in joblet with proper dependency management, network isolation, and resource limits.

## What This Example Does

1. Runs a simple Express.js REST API server
2. Demonstrates npm package installation in isolated environment
3. Shows network isolation and custom network usage
4. Implements health checks and graceful shutdown
5. Includes data persistence with SQLite
6. Shows how to connect multiple services (API + Worker)

## Files

- `server.js` - Express.js API server with REST endpoints
- `worker.js` - Background worker that processes tasks
- `package.json` - Node.js dependencies
- `run-api.sh` - Script to install dependencies and start API
- `run-worker.sh` - Script to start background worker
- `test-api.sh` - Script to test API endpoints

## Prerequisites

**Important**: This example requires Node.js to be pre-installed in the container environment. The default joblet chroot environment is minimal and does not include Node.js. 

To run this example, you would need to:
1. Use a container image with Node.js pre-installed, or
2. Install Node.js in the chroot environment first

## How to Run

### Option 1: Simple API Server

Run a basic REST API server:

```bash
# Upload and run the API server
rnx run --upload-dir=. --network=bridge bash run-api.sh

# Test the API (in another terminal)
rnx run --network=bridge curl http://api:3000/health
```

### Option 2: API with Background Worker

Run API server and worker in the same network:

```bash
# Create a custom network for the application
rnx network create myapp --cidr=10.100.0.0/24

# Start the API server
rnx run --upload-dir=. --network=myapp --max-memory=512 bash run-api.sh

# Start the background worker
rnx run --upload-dir=. --network=myapp --max-memory=256 node worker.js

# Test the full system
rnx run --upload=test-api.sh --network=myapp bash test-api.sh
```

### Option 3: Scheduled Worker

Run a worker that processes tasks periodically:

```bash
# Schedule worker to run every hour
rnx run --schedule="1hour" --upload-dir=. --network=myapp node worker.js
```

## API Endpoints

- `GET /health` - Health check endpoint
- `GET /api/tasks` - List all tasks
- `POST /api/tasks` - Create a new task
- `GET /api/tasks/:id` - Get task by ID
- `PUT /api/tasks/:id` - Update task status
- `DELETE /api/tasks/:id` - Delete a task
- `GET /api/stats` - Get processing statistics

## Example API Usage

```bash
# Create a new task
rnx run --network=myapp curl -X POST http://api:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"title": "Process data", "type": "data-processing"}'

# List all tasks
rnx run --network=myapp curl http://api:3000/api/tasks

# Get statistics
rnx run --network=myapp curl http://api:3000/api/stats
```

## Features Demonstrated

### 🔧 **Dependency Management**
- npm packages installed in isolated environment
- No pollution of host system
- Cached node_modules for faster subsequent runs

### 🌐 **Network Isolation**
- Services communicate within custom network
- Isolated from other joblet jobs
- DNS resolution by service name

### 📊 **Resource Management**
- Memory limits for API and worker
- CPU limits can be added as needed
- Graceful shutdown on resource exhaustion

### 🔄 **Service Communication**
- API creates tasks in SQLite database
- Worker processes tasks asynchronously
- Real-time status updates

### 🛡️ **Security**
- No access to host filesystem (except uploads)
- Network isolation between different apps
- Resource limits prevent DoS

## Production Considerations

For production use, consider:

1. **Persistent Storage**: Mount a volume for SQLite database
2. **Logging**: Use proper logging library (winston, pino)
3. **Monitoring**: Add Prometheus metrics endpoint
4. **Load Balancing**: Run multiple API instances
5. **Environment Variables**: Use `.env` file for configuration

## Troubleshooting

**Port already in use**: The example uses port 3000. If you see binding errors, the port might be in use by another job.

**Cannot connect to API**: Ensure you're using the same network for both server and client.

**npm install fails**: The script will try to install npm if not available. This may take a few minutes on first run.

**Out of memory**: Increase memory limit with `--max-memory=1024` flag.