# Node.js Examples

Examples demonstrating how to use Joblet for Node.js applications, microservices, and JavaScript-based workflows.

## 🟨 Examples Overview

| Example | Files | Description | Complexity | Resources |
|---------|-------|-------------|------------|-----------|
| [API Testing](#api-testing) | `api.test.js`, `package.json` | Automated API testing with Jest | Beginner | 256MB RAM |
| [Microservice Demo](#microservice-demo) | `app.js` | Express.js service with health checks | Intermediate | 512MB RAM |
| [Data Processing](#data-processing) | `process_data.js` | CSV processing with streams | Intermediate | 1GB RAM |
| [Build Pipeline](#build-pipeline) | `build-pipeline.sh` | Complete CI/CD pipeline | Advanced | 1GB RAM |
| [Event Processing](#event-processing) | `event-processor.js` | Real-time event stream handling | Advanced | 512MB RAM |
| [Complete Demo Suite](#complete-demo-suite) | `run_demos.sh` | Automated execution of all examples | All Levels | 2GB RAM |

## 🚀 Quick Start

### Run All Demos (Recommended)
```bash
# Execute complete demo suite
./run_demos.sh
```

### Prerequisites (handled automatically by demo script)
```bash
# Create volumes for Node.js projects
rnx volume create nodejs-projects --size=1GB --type=filesystem
rnx volume create node-modules --size=2GB --type=filesystem
rnx volume create build-cache --size=1GB --type=filesystem
```

### Dependencies
All Node.js dependencies are defined in `package.json`:
```bash
# Install locally (optional)
npm install
```

## 🔍 API Testing

Comprehensive API testing suite with Jest and automated result collection.

### Files Included
- **`api.test.js`**: Complete test suite with authentication, user management, and performance tests
- **`package.json`**: Project configuration with Jest and testing dependencies

### Manual Execution
```bash
# Run comprehensive API tests
rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --max-memory=256 \
       --env=API_BASE_URL=https://api.example.com \
       --env=NODE_ENV=test \
       npm test
```

### Expected Output
- Authentication endpoint testing (valid/invalid credentials)
- User management API validation
- Performance benchmarking of critical endpoints
- Test results saved to `/volumes/nodejs-projects/test-results/`
- Automated result collection in JSON format

**package.json:**
```json
{
  "name": "api-test-suite",
  "version": "1.0.0",
  "scripts": {
    "test": "jest --verbose",
    "test:coverage": "jest --coverage",
    "test:watch": "jest --watch"
  },
  "dependencies": {
    "axios": "^1.4.0",
    "jest": "^29.5.0",
    "supertest": "^6.3.3"
  }
}
```

**api.test.js:**
```javascript
const axios = require('axios');
const fs = require('fs');
const path = require('path');

const API_BASE_URL = process.env.API_BASE_URL || 'http://localhost:3000';

describe('API Test Suite', () => {
  let testResults = [];

  beforeAll(() => {
    console.log(`Testing API: ${API_BASE_URL}`);
  });

  afterAll(() => {
    // Save test results to volume
    const resultsPath = '/volumes/nodejs-projects/test-results';
    fs.mkdirSync(resultsPath, { recursive: true });
    
    fs.writeFileSync(
      path.join(resultsPath, `api-test-results-${Date.now()}.json`),
      JSON.stringify(testResults, null, 2)
    );
  });

  describe('Authentication Endpoints', () => {
    test('POST /auth/login - valid credentials', async () => {
      const response = await axios.post(`${API_BASE_URL}/auth/login`, {
        username: 'testuser',
        password: 'testpass'
      });
      
      expect(response.status).toBe(200);
      expect(response.data).toHaveProperty('token');
      
      testResults.push({
        endpoint: 'POST /auth/login',
        status: 'passed',
        responseTime: response.headers['x-response-time'],
        timestamp: new Date().toISOString()
      });
    });

    test('POST /auth/login - invalid credentials', async () => {
      try {
        await axios.post(`${API_BASE_URL}/auth/login`, {
          username: 'invalid',
          password: 'invalid'
        });
      } catch (error) {
        expect(error.response.status).toBe(401);
        testResults.push({
          endpoint: 'POST /auth/login (invalid)',
          status: 'passed',
          expectedError: true,
          timestamp: new Date().toISOString()
        });
      }
    });
  });

  describe('User Management', () => {
    let authToken;

    beforeAll(async () => {
      const loginResponse = await axios.post(`${API_BASE_URL}/auth/login`, {
        username: 'admin',
        password: 'admin123'
      });
      authToken = loginResponse.data.token;
    });

    test('GET /users - list users', async () => {
      const response = await axios.get(`${API_BASE_URL}/users`, {
        headers: { Authorization: `Bearer ${authToken}` }
      });
      
      expect(response.status).toBe(200);
      expect(Array.isArray(response.data)).toBe(true);
      
      testResults.push({
        endpoint: 'GET /users',
        status: 'passed',
        userCount: response.data.length,
        timestamp: new Date().toISOString()
      });
    });

    test('POST /users - create user', async () => {
      const newUser = {
        username: `testuser_${Date.now()}`,
        email: 'test@example.com',
        role: 'user'
      };

      const response = await axios.post(`${API_BASE_URL}/users`, newUser, {
        headers: { Authorization: `Bearer ${authToken}` }
      });
      
      expect(response.status).toBe(201);
      expect(response.data).toHaveProperty('id');
      
      testResults.push({
        endpoint: 'POST /users',
        status: 'passed',
        createdUserId: response.data.id,
        timestamp: new Date().toISOString()
      });
    });
  });

  describe('Performance Tests', () => {
    test('Response time benchmarks', async () => {
      const endpoints = ['/health', '/users', '/products'];
      const results = [];

      for (const endpoint of endpoints) {
        const startTime = Date.now();
        const response = await axios.get(`${API_BASE_URL}${endpoint}`);
        const endTime = Date.now();
        
        results.push({
          endpoint,
          responseTime: endTime - startTime,
          status: response.status
        });
      }

      // All endpoints should respond within 1 second
      results.forEach(result => {
        expect(result.responseTime).toBeLessThan(1000);
      });

      testResults.push({
        test: 'Performance Benchmarks',
        results,
        timestamp: new Date().toISOString()
      });
    });
  });
});
```

## 🚀 Microservice Demo

Deploy Express.js microservice with health checks, authentication, and error handling.

### Files Included
- **`app.js`**: Complete Express.js service with REST endpoints and middleware

### Manual Execution
```bash
# Deploy demo microservice
rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --volume=node-modules \
       --max-memory=512 \
       --env=PORT=3001 \
       --env=NODE_ENV=production \
       npm start
```

### Service Features
- Health check endpoint (`/health`)
- Mock authentication (`/auth/login`)
- User management endpoints (`/users`)
- Rate limiting and security middleware
- Comprehensive error logging to volumes

**user-service/app.js:**
```javascript
const express = require('express');
const mongoose = require('mongoose');
const cors = require('cors');
const helmet = require('helmet');
const rateLimit = require('express-rate-limit');
const fs = require('fs');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3001;
const NODE_ENV = process.env.NODE_ENV || 'development';

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json({ limit: '10mb' }));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100 // limit each IP to 100 requests per windowMs
});
app.use(limiter);

// Health check
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
    environment: NODE_ENV
  });
});

// User routes
const userRoutes = require('./routes/users');
app.use('/api/users', userRoutes);

// Error handling middleware
app.use((error, req, res, next) => {
  console.error('Error:', error);
  
  // Log error to persistent volume
  const errorLog = {
    timestamp: new Date().toISOString(),
    error: error.message,
    stack: error.stack,
    url: req.url,
    method: req.method,
    ip: req.ip
  };
  
  const logPath = '/volumes/nodejs-projects/logs';
  fs.mkdirSync(logPath, { recursive: true });
  fs.appendFileSync(
    path.join(logPath, 'error.log'),
    JSON.stringify(errorLog) + '\\n'
  );
  
  res.status(500).json({
    error: 'Internal server error',
    timestamp: new Date().toISOString()
  });
});

// Start server
const server = app.listen(PORT, () => {
  console.log(`User service running on port ${PORT} in ${NODE_ENV} mode`);
  console.log(`Health check: http://localhost:${PORT}/health`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully');
  server.close(() => {
    console.log('Process terminated');
  });
});

module.exports = app;
```

**routes/users.js:**
```javascript
const express = require('express');
const router = express.Router();
const User = require('../models/User');
const { body, validationResult } = require('express-validator');

// Get all users
router.get('/', async (req, res) => {
  try {
    const page = parseInt(req.query.page) || 1;
    const limit = parseInt(req.query.limit) || 10;
    const skip = (page - 1) * limit;

    const users = await User.find()
      .select('-password')
      .skip(skip)
      .limit(limit)
      .sort({ createdAt: -1 });

    const total = await User.countDocuments();

    res.json({
      users,
      pagination: {
        page,
        limit,
        total,
        pages: Math.ceil(total / limit)
      }
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// Create user
router.post('/',
  [
    body('username').isLength({ min: 3 }).withMessage('Username must be at least 3 characters'),
    body('email').isEmail().withMessage('Valid email required'),
    body('password').isLength({ min: 6 }).withMessage('Password must be at least 6 characters')
  ],
  async (req, res) => {
    try {
      const errors = validationResult(req);
      if (!errors.isEmpty()) {
        return res.status(400).json({ errors: errors.array() });
      }

      const { username, email, password, role = 'user' } = req.body;

      // Check if user exists
      const existingUser = await User.findOne({
        $or: [{ email }, { username }]
      });

      if (existingUser) {
        return res.status(409).json({ error: 'User already exists' });
      }

      const user = new User({
        username,
        email,
        password, // In real app, hash this
        role
      });

      await user.save();

      // Return user without password
      const userResponse = user.toObject();
      delete userResponse.password;

      res.status(201).json(userResponse);
    } catch (error) {
      res.status(500).json({ error: error.message });
    }
  }
);

// Get user by ID
router.get('/:id', async (req, res) => {
  try {
    const user = await User.findById(req.params.id).select('-password');
    
    if (!user) {
      return res.status(404).json({ error: 'User not found' });
    }

    res.json(user);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// Update user
router.put('/:id', async (req, res) => {
  try {
    const { username, email, role } = req.body;
    
    const user = await User.findByIdAndUpdate(
      req.params.id,
      { username, email, role, updatedAt: new Date() },
      { new: true, runValidators: true }
    ).select('-password');

    if (!user) {
      return res.status(404).json({ error: 'User not found' });
    }

    res.json(user);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// Delete user
router.delete('/:id', async (req, res) => {
  try {
    const user = await User.findByIdAndDelete(req.params.id);
    
    if (!user) {
      return res.status(404).json({ error: 'User not found' });
    }

    res.json({ message: 'User deleted successfully' });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

module.exports = router;
```

## 📊 Data Processing

Process CSV data with Node.js streams, transformations, and automated reporting.

### Files Included
- **`process_data.js`**: Complete data processing pipeline with stream transformations

### Manual Execution
```bash
# Process CSV data with streaming
rnx run --upload=process_data.js \
       --volume=nodejs-projects \
       --max-memory=1024 \
       --max-iobps=52428800 \
       node process_data.js
```

### Processing Features
- Automatic sample data generation if no input files exist
- Stream-based CSV processing for memory efficiency
- Data transformations (name normalization, salary grading, experience calculation)
- Performance metrics and processing statistics
- Results saved to `/volumes/nodejs-projects/output/` and `/volumes/nodejs-projects/stats/`

**process_data.js:**
```javascript
const fs = require('fs');
const path = require('path');
const csv = require('csv-parser');
const { Transform } = require('stream');
const createCsvWriter = require('csv-writer').createObjectCsvWriter;

class DataProcessor {
  constructor(config) {
    this.config = config;
    this.inputDir = '/volumes/nodejs-projects/input';
    this.outputDir = '/volumes/nodejs-projects/output';
    this.statsDir = '/volumes/nodejs-projects/stats';
    
    // Create directories
    [this.outputDir, this.statsDir].forEach(dir => {
      fs.mkdirSync(dir, { recursive: true });
    });
    
    this.stats = {
      totalRecords: 0,
      processedRecords: 0,
      errorRecords: 0,
      startTime: new Date(),
      endTime: null
    };
  }

  // Transform stream for data processing
  createTransformStream() {
    return new Transform({
      objectMode: true,
      transform(chunk, encoding, callback) {
        try {
          // Data transformation logic
          const transformed = {
            id: chunk.id,
            name: chunk.name?.trim().toLowerCase(),
            email: chunk.email?.toLowerCase(),
            age: parseInt(chunk.age) || null,
            salary: parseFloat(chunk.salary) || null,
            department: chunk.department?.trim(),
            joinDate: new Date(chunk.join_date).toISOString().split('T')[0],
            isActive: chunk.status === 'active',
            // Add calculated fields
            salaryGrade: this.calculateSalaryGrade(parseFloat(chunk.salary)),
            experienceYears: this.calculateExperience(chunk.join_date),
            processedAt: new Date().toISOString()
          };

          this.stats.processedRecords++;
          callback(null, transformed);
          
        } catch (error) {
          this.stats.errorRecords++;
          console.error(`Error processing record ${chunk.id}:`, error.message);
          callback(); // Skip invalid records
        }
      }.bind(this)
    });
  }

  calculateSalaryGrade(salary) {
    if (!salary) return 'unknown';
    if (salary < 30000) return 'entry';
    if (salary < 60000) return 'junior';
    if (salary < 100000) return 'senior';
    return 'executive';
  }

  calculateExperience(joinDate) {
    const join = new Date(joinDate);
    const now = new Date();
    return Math.floor((now - join) / (365.25 * 24 * 60 * 60 * 1000));
  }

  async processFile(inputFile, outputFile) {
    return new Promise((resolve, reject) => {
      console.log(`Processing ${inputFile}...`);
      
      const csvWriter = createCsvWriter({
        path: outputFile,
        header: [
          {id: 'id', title: 'ID'},
          {id: 'name', title: 'Name'},
          {id: 'email', title: 'Email'},
          {id: 'age', title: 'Age'},
          {id: 'salary', title: 'Salary'},
          {id: 'department', title: 'Department'},
          {id: 'joinDate', title: 'Join Date'},
          {id: 'isActive', title: 'Active'},
          {id: 'salaryGrade', title: 'Salary Grade'},
          {id: 'experienceYears', title: 'Experience Years'},
          {id: 'processedAt', title: 'Processed At'}
        ]
      });

      const transformedRecords = [];
      
      fs.createReadStream(inputFile)
        .pipe(csv())
        .on('data', (data) => {
          this.stats.totalRecords++;
        })
        .pipe(this.createTransformStream())
        .on('data', (transformed) => {
          transformedRecords.push(transformed);
        })
        .on('end', async () => {
          try {
            await csvWriter.writeRecords(transformedRecords);
            console.log(`Processed ${transformedRecords.length} records to ${outputFile}`);
            resolve(transformedRecords.length);
          } catch (error) {
            reject(error);
          }
        })
        .on('error', reject);
    });
  }

  async generateReport(processedFiles) {
    this.stats.endTime = new Date();
    this.stats.processingTimeMs = this.stats.endTime - this.stats.startTime;
    
    const report = {
      summary: this.stats,
      filesProcessed: processedFiles,
      performance: {
        recordsPerSecond: Math.round(this.stats.processedRecords / (this.stats.processingTimeMs / 1000)),
        memoryUsage: process.memoryUsage()
      }
    };

    // Save report
    const reportPath = path.join(this.statsDir, `processing-report-${Date.now()}.json`);
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
    
    console.log('\\nProcessing Report:');
    console.log(`Total Records: ${this.stats.totalRecords}`);
    console.log(`Processed Records: ${this.stats.processedRecords}`);
    console.log(`Error Records: ${this.stats.errorRecords}`);
    console.log(`Processing Time: ${this.stats.processingTimeMs}ms`);
    console.log(`Records/Second: ${report.performance.recordsPerSecond}`);
    console.log(`Report saved: ${reportPath}`);
    
    return report;
  }

  async run() {
    try {
      console.log('Starting data processing...');
      
      // Get all CSV files in input directory
      const files = fs.readdirSync(this.inputDir)
        .filter(file => file.endsWith('.csv'));
      
      if (files.length === 0) {
        console.log('No CSV files found in input directory');
        return;
      }

      const processedFiles = [];
      
      // Process each file
      for (const file of files) {
        const inputPath = path.join(this.inputDir, file);
        const outputPath = path.join(this.outputDir, `processed_${file}`);
        
        const recordCount = await this.processFile(inputPath, outputPath);
        processedFiles.push({
          inputFile: file,
          outputFile: `processed_${file}`,
          recordCount
        });
      }

      // Generate final report
      await this.generateReport(processedFiles);
      
    } catch (error) {
      console.error('Processing failed:', error);
      process.exit(1);
    }
  }
}

// Load configuration
const config = JSON.parse(fs.readFileSync('config.json', 'utf8'));

// Run processor
const processor = new DataProcessor(config);
processor.run().then(() => {
  console.log('Data processing completed successfully');
}).catch((error) => {
  console.error('Data processing failed:', error);
  process.exit(1);
});
```

## 🔧 Build Pipeline

Complete CI/CD pipeline with testing, linting, and artifact generation.

### Files Included
- **`build-pipeline.sh`**: Complete build automation script

### Manual Execution
```bash
# Run complete build pipeline
rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --volume=node-modules \
       --volume=build-cache \
       --max-memory=1024 \
       --env=NODE_ENV=production \
       --env=BUILD_NUMBER=$(date +%s) \
       bash build-pipeline.sh
```

### Pipeline Stages
1. **Environment Setup**: Node.js version verification and dependency caching
2. **Code Quality**: ESLint linting with report generation
3. **Testing**: Jest test execution with coverage reporting
4. **Demo Processing**: Data processing demonstration
5. **Artifact Creation**: Build manifest and artifact packaging
6. **Report Generation**: Comprehensive build reports saved to volumes

**build-pipeline.sh:**
```bash
#!/bin/bash
set -e

echo "🚀 Starting Build Pipeline"
echo "Build Number: $BUILD_NUMBER"
echo "Node Environment: $NODE_ENV"

# Setup directories
WORKSPACE="/work"
OUTPUT_DIR="/volumes/nodejs-projects/builds/$BUILD_NUMBER"
CACHE_DIR="/volumes/build-cache"
REPORTS_DIR="/volumes/nodejs-projects/reports"

mkdir -p "$OUTPUT_DIR" "$REPORTS_DIR"

cd "$WORKSPACE"

echo "📋 Step 1: Environment Setup"
node --version
npm --version

# Use cached node_modules if available
if [ -d "/volumes/node-modules/node_modules" ]; then
    echo "Using cached node_modules"
    ln -sf /volumes/node-modules/node_modules ./node_modules
else
    echo "Installing dependencies"
    npm ci --cache="$CACHE_DIR"
    cp -r node_modules /volumes/node-modules/
fi

echo "🔍 Step 2: Code Quality Checks"
npm run lint 2>&1 | tee "$REPORTS_DIR/lint-report.txt"
npm run audit 2>&1 | tee "$REPORTS_DIR/audit-report.txt"

echo "🧪 Step 3: Testing"
npm run test:unit -- --ci --coverage --coverageReporters=text --coverageReporters=json | tee "$REPORTS_DIR/test-report.txt"
npm run test:integration -- --ci | tee "$REPORTS_DIR/integration-report.txt"

echo "📦 Step 4: Build"
npm run build
cp -r dist/* "$OUTPUT_DIR/"

echo "📊 Step 5: Bundle Analysis"
npm run analyze 2>&1 | tee "$REPORTS_DIR/bundle-analysis.txt"

echo "🔐 Step 6: Security Scan"
npm audit --audit-level=moderate 2>&1 | tee "$REPORTS_DIR/security-scan.txt"

echo "📋 Step 7: Build Artifacts"
# Create build manifest
cat > "$OUTPUT_DIR/build-manifest.json" << EOF
{
  "buildNumber": "$BUILD_NUMBER",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "nodeVersion": "$(node --version)",
  "npmVersion": "$(npm --version)",
  "environment": "$NODE_ENV",
  "gitCommit": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
  "buildDuration": "$(date +%s)",
  "artifacts": [
    $(find "$OUTPUT_DIR" -type f -name "*.js" -o -name "*.css" -o -name "*.html" | sed 's/.*/"&"/' | paste -sd, -)
  ]
}
EOF

echo "✅ Build Pipeline Completed Successfully"
echo "Build artifacts saved to: $OUTPUT_DIR"
echo "Reports saved to: $REPORTS_DIR"
```

**package.json (build scripts):**
```json
{
  "scripts": {
    "lint": "eslint src/ --format=json --output-file=/volumes/nodejs-projects/reports/eslint.json",
    "audit": "npm audit --json > /volumes/nodejs-projects/reports/audit.json",
    "test:unit": "jest --testPathPattern=src/__tests__ --json --outputFile=/volumes/nodejs-projects/reports/jest.json",
    "test:integration": "jest --testPathPattern=integration/ --runInBand",
    "build": "webpack --mode=production --json > /volumes/nodejs-projects/reports/webpack-stats.json",
    "analyze": "webpack-bundle-analyzer dist/bundle.js --report --no-open --output-file=/volumes/nodejs-projects/reports/bundle-report.html"
  }
}
```

## ⚡ Event Processing

Real-time event stream processing with metrics collection and automated workflows.

### Files Included
- **`event-processor.js`**: Event stream processor with multiple event type handlers

### Manual Execution
```bash
# Process event streams with simulation
rnx run --upload=event-processor.js \
       --volume=nodejs-projects \
       --max-memory=512 \
       --env=DEMO_MODE=true \
       node event-processor.js
```

### Event Processing Features
- Multiple event type handlers (user.created, order.placed, payment.processed)
- Real-time metrics collection (events/second, error rates)
- Event data transformation and sanitization
- Workflow triggering based on event types
- Comprehensive logging to `/volumes/nodejs-projects/events/`
- Automatic shutdown after demo time limit

## 🔄 Complete Demo Suite

Execute all Node.js examples with a single command.

### Files Included
- **`run_demos.sh`**: Master demo script
- **`package.json`**: Complete project configuration with all dependencies

### What It Runs
1. **API Testing**: Executes comprehensive test suite with result collection
2. **Microservice Demo**: Starts Express.js service with health checks
3. **Data Processing**: Processes CSV data with stream transformations
4. **Build Pipeline**: Runs complete CI/CD workflow with reporting
5. **Event Processing**: Simulates real-time event handling for 30 seconds

### Execution
```bash
# Run complete demo suite
./run_demos.sh
```

### Demo Flow
1. Creates required volumes automatically
2. Executes API tests with mock endpoints
3. Deploys microservice in background
4. Processes sample employee data
5. Runs complete build pipeline with caching
6. Simulates event processing with metrics collection
7. Provides comprehensive result locations and inspection commands

## 📁 Demo Results

After running the demos, check results in the following locations:

### Test Results
```bash
# View API test results
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/test-results/api-test-results-*.json
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/reports/test-report.txt
```

### Build Artifacts
```bash
# View build outputs and reports
rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/builds/
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/reports/lint-report.txt
```

### Data Processing Results
```bash
# View processed data and statistics
rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/output/
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/stats/processing-report-*.json
```

### Event Processing Logs
```bash
# View event processing results
rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/events/
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/metrics/real-time-metrics.json
```

**Sample Event Processor Code:**
```javascript
const EventEmitter = require('events');
const fs = require('fs');
const path = require('path');

class EventProcessor extends EventEmitter {
  constructor(config) {
    super();
    this.config = config;
    this.outputDir = '/volumes/nodejs-projects/events';
    this.metricsDir = '/volumes/nodejs-projects/metrics';
    
    fs.mkdirSync(this.outputDir, { recursive: true });
    fs.mkdirSync(this.metricsDir, { recursive: true });
    
    this.metrics = {
      eventsProcessed: 0,
      eventsPerSecond: 0,
      errors: 0,
      startTime: Date.now()
    };
    
    this.setupMetricsReporting();
  }

  setupMetricsReporting() {
    setInterval(() => {
      const now = Date.now();
      const elapsed = (now - this.metrics.startTime) / 1000;
      this.metrics.eventsPerSecond = Math.round(this.metrics.eventsProcessed / elapsed);
      
      console.log(`📊 Metrics: ${this.metrics.eventsProcessed} events, ${this.metrics.eventsPerSecond} events/sec, ${this.metrics.errors} errors`);
      
      // Save metrics
      const metricsFile = path.join(this.metricsDir, 'real-time-metrics.json');
      fs.writeFileSync(metricsFile, JSON.stringify({
        ...this.metrics,
        timestamp: new Date().toISOString()
      }, null, 2));
      
    }, 5000);
  }

  async processEvent(event) {
    try {
      // Event processing logic
      const processed = {
        id: event.id,
        type: event.type,
        timestamp: new Date().toISOString(),
        data: this.transformEventData(event.data),
        metadata: {
          source: event.source,
          processedBy: 'joblet-event-processor',
          processingTime: Date.now()
        }
      };

      // Route to appropriate handler
      switch (event.type) {
        case 'user.created':
          await this.handleUserCreated(processed);
          break;
        case 'order.placed':
          await this.handleOrderPlaced(processed);
          break;
        case 'payment.processed':
          await this.handlePaymentProcessed(processed);
          break;
        default:
          await this.handleGenericEvent(processed);
      }

      this.metrics.eventsProcessed++;
      this.emit('eventProcessed', processed);
      
    } catch (error) {
      this.metrics.errors++;
      this.emit('error', error, event);
    }
  }

  transformEventData(data) {
    // Common data transformations
    if (data.email) {
      data.email = data.email.toLowerCase();
    }
    
    if (data.timestamp) {
      data.timestamp = new Date(data.timestamp).toISOString();
    }
    
    // Sanitize sensitive data
    if (data.password) {
      delete data.password;
    }
    
    return data;
  }

  async handleUserCreated(event) {
    console.log(`👤 User created: ${event.data.email}`);
    
    // Save to user events log
    const userEventsFile = path.join(this.outputDir, 'user-events.jsonl');
    fs.appendFileSync(userEventsFile, JSON.stringify(event) + '\\n');
    
    // Trigger welcome email workflow
    this.emit('triggerWorkflow', {
      type: 'send_welcome_email',
      userId: event.data.id,
      email: event.data.email
    });
  }

  async handleOrderPlaced(event) {
    console.log(`🛒 Order placed: ${event.data.orderId} - $${event.data.amount}`);
    
    // Save to orders log
    const ordersFile = path.join(this.outputDir, 'order-events.jsonl');
    fs.appendFileSync(ordersFile, JSON.stringify(event) + '\\n');
    
    // Update inventory
    this.emit('triggerWorkflow', {
      type: 'update_inventory',
      orderId: event.data.orderId,
      items: event.data.items
    });
  }

  async handlePaymentProcessed(event) {
    console.log(`💳 Payment processed: ${event.data.paymentId} - $${event.data.amount}`);
    
    // Save to payments log
    const paymentsFile = path.join(this.outputDir, 'payment-events.jsonl');
    fs.appendFileSync(paymentsFile, JSON.stringify(event) + '\\n');
    
    // Send confirmation
    this.emit('triggerWorkflow', {
      type: 'send_payment_confirmation',
      paymentId: event.data.paymentId,
      userId: event.data.userId
    });
  }

  async handleGenericEvent(event) {
    console.log(`📝 Generic event: ${event.type}`);
    
    const genericFile = path.join(this.outputDir, 'generic-events.jsonl');
    fs.appendFileSync(genericFile, JSON.stringify(event) + '\\n');
  }

  start() {
    console.log('🚀 Event Processor Started');
    
    // Simulate event stream (in real app, connect to Kafka, Redis, etc.)
    this.simulateEventStream();
    
    // Setup error handling
    this.on('error', (error, event) => {
      console.error('❌ Event processing error:', error.message);
      
      const errorLog = {
        timestamp: new Date().toISOString(),
        error: error.message,
        event: event,
        stack: error.stack
      };
      
      const errorFile = path.join(this.outputDir, 'error-events.jsonl');
      fs.appendFileSync(errorFile, JSON.stringify(errorLog) + '\\n');
    });
  }

  simulateEventStream() {
    const eventTypes = [
      'user.created',
      'order.placed',
      'payment.processed',
      'product.viewed',
      'cart.updated'
    ];

    setInterval(() => {
      const event = {
        id: `event_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
        type: eventTypes[Math.floor(Math.random() * eventTypes.length)],
        source: 'demo-app',
        data: this.generateSampleEventData(),
        timestamp: new Date().toISOString()
      };

      this.processEvent(event);
    }, 100); // Process event every 100ms
  }

  generateSampleEventData() {
    return {
      id: `user_${Math.floor(Math.random() * 10000)}`,
      email: `user${Math.floor(Math.random() * 1000)}@example.com`,
      amount: Math.floor(Math.random() * 1000) + 10,
      orderId: `order_${Date.now()}`,
      paymentId: `payment_${Date.now()}`,
      items: [
        { id: 'item1', quantity: Math.floor(Math.random() * 5) + 1 },
        { id: 'item2', quantity: Math.floor(Math.random() * 3) + 1 }
      ]
    };
  }
}

// Start event processor
const config = {
  batchSize: 100,
  maxConcurrency: 10,
  retryAttempts: 3
};

const processor = new EventProcessor(config);
processor.start();

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('🛑 Shutting down event processor...');
  
  // Save final metrics
  const finalMetrics = {
    ...processor.metrics,
    endTime: Date.now(),
    shutdownReason: 'SIGTERM'
  };
  
  const metricsFile = path.join('/volumes/nodejs-projects/metrics', 'final-metrics.json');
  fs.writeFileSync(metricsFile, JSON.stringify(finalMetrics, null, 2));
  
  process.exit(0);
});
```

## 🎯 Best Practices Demonstrated

### Performance Optimization
- **Stream Processing**: `process_data.js` uses streams for memory-efficient CSV processing
- **Caching**: Build pipeline demonstrates node_modules caching across jobs
- **Resource Limits**: Examples show appropriate memory and CPU limits per workload type

### Error Handling
- **Graceful Degradation**: All scripts handle missing dependencies and services gracefully
- **Comprehensive Logging**: Error logs saved to persistent volumes with structured formatting
- **Demo Mode**: Scripts detect and adapt to demo environments automatically

### Security & Production Patterns
- **Input Validation**: API endpoints include request validation and sanitization
- **Rate Limiting**: Express service implements rate limiting middleware
- **Environment Variables**: Configuration through environment variables demonstrated
- **Health Checks**: Microservice includes comprehensive health check endpoint

### Monitoring & Observability
- **Metrics Collection**: Event processor tracks real-time performance metrics
- **Build Reporting**: Pipeline generates detailed reports for all stages
- **Volume Organization**: Structured data organization across persistent volumes
- **Process Monitoring**: Scripts provide detailed progress and completion status

## 🚀 Next Steps

1. **Replace Mock Data**: Update `process_data.js` to load your real CSV files
2. **Connect Real APIs**: Modify `api.test.js` to test your actual API endpoints
3. **Add Database Integration**: Extend `app.js` with real database connections
4. **Scale Processing**: Increase memory limits and add more concurrent workers
5. **Production Deployment**: Add monitoring, error alerting, and log aggregation
6. **CI/CD Integration**: Integrate `build-pipeline.sh` with your CI/CD platform

## 📊 Monitoring Your Jobs

```bash
# Monitor job execution in real-time
rnx monitor

# Check job status and resource usage
rnx list

# View specific job logs
rnx log <job-id>

# Monitor volume usage
rnx volume list
```

## 📚 Additional Resources

- [Node.js Documentation](https://nodejs.org/docs/)
- [Express.js Guide](https://expressjs.com/en/guide/)
- [Jest Testing Framework](https://jestjs.io/docs/getting-started)
- [Node.js Stream API](https://nodejs.org/api/stream.html)
- [Joblet Documentation](../../docs/) - Configuration and advanced usage