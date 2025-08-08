# Runtime System Examples

This document provides practical examples of using the Joblet runtime system across different languages and scenarios.

## Basic Runtime Usage

### Python Examples

#### Data Analysis with ML Runtime
```bash
# Upload data file and run analysis
rnx run --runtime=python:3.11+ml \
        --upload=data.csv \
        --volume=analysis-results \
        python -c "
import pandas as pd
import numpy as np
data = pd.read_csv('data.csv')
print(f'Dataset shape: {data.shape}')
print(data.describe())
data.to_json('/volumes/analysis-results/summary.json')
"
```

#### Modern Python Features
```bash
# Using Python 3.12 modern syntax
rnx run --runtime=python:3.12 \
        --upload=modern_app.py \
        python modern_app.py
```

#### Web API Development
```bash
# Flask web application with external access
rnx run --runtime=python:3.11+ml \
        --upload=api.py \
        --network=web \
        --max-memory=512 \
        python api.py
```

### Java Examples

#### Enterprise Application (Java 17)
```bash
# Compile and run Java application
rnx run --runtime=java:17 \
        --upload=Application.java \
        --volume=maven-cache \
        bash -c "javac Application.java && java Application"
```

#### Modern Java with Virtual Threads (Java 21)
```bash
# High-concurrency application using Virtual Threads
rnx run --runtime=java:21 \
        --upload=VirtualThreadApp.java \
        --max-memory=1024 \
        bash -c "javac VirtualThreadApp.java && java VirtualThreadApp"
```

#### Maven Project Build
```bash
# Build entire Maven project
rnx run --runtime=java:17 \
        --upload-dir=spring-project \
        --volume=maven-cache \
        --max-memory=2048 \
        mvn clean package
```

### Node.js Examples

#### Express Web Application
```bash
# Run Express server with external access
rnx run --runtime=node:18 \
        --upload=server.js \
        --upload=package.json \
        --network=web \
        --volume=npm-cache \
        bash -c "npm install && node server.js"
```

#### TypeScript Development
```bash
# Compile and run TypeScript
rnx run --runtime=node:18 \
        --upload=app.ts \
        --upload=tsconfig.json \
        bash -c "tsc app.ts && node app.js"
```

## Advanced Runtime Scenarios

### Multi-Stage Processing Pipeline

#### Stage 1: Data Preparation (Python)
```bash
# Prepare and clean data
rnx run --runtime=python:3.11+ml \
        --upload=raw_data.csv \
        --volume=pipeline-data \
        python -c "
import pandas as pd
data = pd.read_csv('raw_data.csv')
cleaned = data.dropna().reset_index(drop=True)
cleaned.to_csv('/volumes/pipeline-data/cleaned.csv', index=False)
print(f'Cleaned dataset: {len(cleaned)} rows')
"
```

#### Stage 2: Analysis (Python ML)
```bash
# Run machine learning analysis
rnx run --runtime=python:3.11+ml \
        --volume=pipeline-data \
        --max-memory=2048 \
        python -c "
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
import joblib

data = pd.read_csv('/volumes/pipeline-data/cleaned.csv')
# ... ML processing ...
joblib.dump(model, '/volumes/pipeline-data/model.pkl')
"
```

#### Stage 3: Report Generation (Java)
```bash
# Generate PDF reports
rnx run --runtime=java:17 \
        --volume=pipeline-data \
        --upload=ReportGenerator.java \
        bash -c "
javac ReportGenerator.java
java ReportGenerator /volumes/pipeline-data/model.pkl
"
```

### Development Workflows

#### Python Package Development
```bash
# Test package in isolated environment
rnx run --runtime=python:3.12 \
        --upload-dir=my-package \
        --volume=dev-pip-cache \
        bash -c "
cd my-package
pip install -e .
python -m pytest tests/
python setup.py sdist
"
```

#### Java Library Testing
```bash
# Multi-version compatibility testing
# Test on Java 17
rnx run --runtime=java:17 \
        --upload-dir=java-library \
        --volume=test-results \
        bash -c "
cd java-library
mvn test
cp target/surefire-reports/* /volumes/test-results/java17-
"

# Test on Java 21
rnx run --runtime=java:21 \
        --upload-dir=java-library \
        --volume=test-results \
        bash -c "
cd java-library
mvn test
cp target/surefire-reports/* /volumes/test-results/java21-
"
```

### Web Development Examples

#### Full-Stack Development Server
```bash
# Frontend build (Node.js)
rnx run --runtime=node:18 \
        --upload-dir=frontend \
        --volume=frontend-dist \
        bash -c "
cd frontend
npm install
npm run build
cp -r dist/* /volumes/frontend-dist/
"

# Backend API (Python)
rnx run --runtime=python:3.11+ml \
        --upload-dir=backend \
        --volume=frontend-dist \
        --network=web \
        python -c "
from flask import Flask, send_from_directory
app = Flask(__name__)

@app.route('/')
def frontend():
    return send_from_directory('/volumes/frontend-dist', 'index.html')

@app.route('/api/data')
def api():
    return {'message': 'Hello from Python API'}

app.run(host='0.0.0.0', port=8080)
"
```

### Database Integration Examples

#### Python Database Analysis
```bash
# Connect to database and analyze
rnx run --runtime=python:3.11+ml \
        --network=database \
        --volume=db-cache \
        python -c "
import pandas as pd
import psycopg2
from sqlalchemy import create_engine

engine = create_engine('postgresql://user:pass@db:5432/analytics')
data = pd.read_sql('SELECT * FROM sales', engine)
summary = data.groupby('region').sum()
summary.to_csv('/volumes/db-cache/region_summary.csv')
"
```

### Batch Processing Examples

#### Large File Processing
```bash
# Process large CSV files in chunks
rnx run --runtime=python:3.11+ml \
        --upload=large_dataset.csv \
        --volume=processed-chunks \
        --max-memory=4096 \
        --max-cpu=4 \
        python -c "
import pandas as pd
import numpy as np

chunk_size = 10000
chunk_num = 0

for chunk in pd.read_csv('large_dataset.csv', chunksize=chunk_size):
    processed = chunk.apply(lambda x: x.str.upper() if x.dtype == 'object' else x)
    processed.to_csv(f'/volumes/processed-chunks/chunk_{chunk_num}.csv', index=False)
    chunk_num += 1
    print(f'Processed chunk {chunk_num}')
"
```

### Cross-Runtime Communication

#### Message Queue Processing
```bash
# Producer (Python)
rnx run --runtime=python:3.11+ml \
        --network=message-queue \
        python -c "
import json
import time
import requests

for i in range(100):
    message = {'id': i, 'data': f'message {i}'}
    requests.post('http://queue:8080/messages', json=message)
    time.sleep(0.1)
"

# Consumer (Node.js)
rnx run --runtime=node:18 \
        --network=message-queue \
        --upload=consumer.js \
        node consumer.js
```

## Performance Optimization Examples

### Memory-Optimized Processing
```bash
# Large dataset processing with memory constraints
rnx run --runtime=python:3.11+ml \
        --upload=big_data.csv \
        --max-memory=1024 \
        --volume=temp-storage \
        python -c "
import pandas as pd
import gc

# Process in chunks to manage memory
def process_large_file(filename):
    chunk_iter = pd.read_csv(filename, chunksize=1000)
    results = []
    
    for chunk in chunk_iter:
        result = chunk.groupby('category').sum()
        results.append(result)
        
        # Force garbage collection
        gc.collect()
    
    final_result = pd.concat(results).groupby(level=0).sum()
    final_result.to_csv('/volumes/temp-storage/summary.csv')
    
process_large_file('big_data.csv')
"
```

### CPU-Intensive Processing
```bash
# Multi-core processing
rnx run --runtime=python:3.11+ml \
        --upload=compute_task.py \
        --max-cpu=8 \
        --max-memory=4096 \
        python -c "
from multiprocessing import Pool
import numpy as np

def cpu_intensive_task(data_chunk):
    # Simulate heavy computation
    return np.fft.fft(data_chunk).sum()

if __name__ == '__main__':
    # Generate test data
    data = np.random.random(1000000)
    chunks = np.array_split(data, 8)
    
    with Pool(processes=8) as pool:
        results = pool.map(cpu_intensive_task, chunks)
    
    print(f'Final result: {sum(results)}')
"
```

## Testing and Validation Examples

### Runtime Compatibility Testing
```bash
# Test script across multiple Python versions
for runtime in python:3.11+ml python:3.12; do
    echo "Testing on $runtime"
    rnx run --runtime=$runtime \
            --upload=compatibility_test.py \
            python compatibility_test.py
done
```

### Integration Testing
```bash
# Test complete application stack
# Database setup
rnx run --runtime=python:3.11+ml \
        --network=test-net \
        --volume=test-db \
        python setup_test_db.py

# API testing
rnx run --runtime=python:3.11+ml \
        --network=test-net \
        --upload=test_api.py \
        python test_api.py

# Frontend testing
rnx run --runtime=node:18 \
        --network=test-net \
        --upload-dir=frontend-tests \
        bash -c "cd frontend-tests && npm test"
```

## Monitoring and Debugging Examples

### Performance Monitoring
```bash
# Monitor resource usage during job execution
rnx run --runtime=python:3.11+ml \
        --upload=heavy_computation.py \
        --monitor=memory,cpu \
        --max-runtime=300 \
        python heavy_computation.py
```

### Debug Mode Execution
```bash
# Run with debug logging and extended timeout
rnx run --runtime=java:21 \
        --upload=DebugApp.java \
        --debug \
        --max-runtime=600 \
        bash -c "
javac -g DebugApp.java
java -Xdebug -Xrunjdwp:transport=dt_socket,server=y,suspend=n,address=5005 DebugApp
"
```

## Migration Examples

### Legacy Python to Runtime
```bash
# Before (host-dependent)
python3 my_script.py

# After (runtime-isolated)
rnx run --runtime=python:3.11+ml \
        --upload=my_script.py \
        python my_script.py
```

### Complex Application Migration
```bash
# Legacy complex deployment
# sudo apt install python3.11 python3-pip
# pip3 install pandas numpy
# python3 app.py

# New runtime-based deployment
rnx run --runtime=python:3.11+ml \
        --upload=app.py \
        --upload=requirements.txt \
        --volume=app-data \
        --network=web \
        python app.py
```

## Best Practices Examples

### Resource Management
```bash
# Appropriate resource allocation
rnx run --runtime=python:3.11+ml \
        --max-memory=2048 \      # 2GB for ML workloads
        --max-cpu=4 \            # 4 cores for parallel processing
        --max-runtime=3600 \     # 1 hour timeout
        --upload=ml_training.py \
        python ml_training.py
```

### Volume Usage
```bash
# Persistent data and cache management
rnx run --runtime=node:18 \
        --volume=npm-cache \     # Persistent npm cache
        --volume=project-data \  # Persistent project data
        --upload-dir=node-app \
        bash -c "
cd node-app
npm install  # Uses cached packages
npm start
"
```

### Network Isolation
```bash
# Secure network access patterns
rnx run --runtime=python:3.11+ml \
        --network=database \     # Access only to database
        --upload=data_processor.py \
        python data_processor.py

# Web service with controlled access
rnx run --runtime=node:18 \
        --network=web \          # External web access
        --upload=api_server.js \
        node api_server.js
```