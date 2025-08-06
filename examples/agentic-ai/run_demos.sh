#!/bin/bash
set -e

echo "🤖 Setting up Agentic AI Demo Environment"
echo "==========================================="

# System compatibility checks
echo "🔍 Checking system compatibility..."

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    echo "Please ensure Joblet RNX client is installed and in PATH"
    exit 1
fi

# Test connection to Joblet server
echo "🔗 Testing connection to Joblet server..."
if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    echo "Please ensure Joblet daemon is running and RNX is configured"
    exit 1
fi

# Check system resources for AI workloads
echo "📊 Checking available system resources..."
if command -v free &> /dev/null; then
    AVAILABLE_RAM=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    if [ "$AVAILABLE_RAM" -lt 4096 ]; then
        echo "⚠️  Warning: Available RAM (${AVAILABLE_RAM}MB) may be insufficient for AI demos"
        echo "   Recommended: 8GB+ available RAM for optimal AI performance"
        echo "   Some demos may fail with insufficient memory"
    fi
fi

# Check if we have the required demo files
echo "📁 Checking AI demo files..."
REQUIRED_FILES=("llm_inference.py" "multi_agent_system.py" "rag_system.py" "distributed_training.py" "requirements.txt")
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "❌ Error: Required file '$file' not found"
        echo "Please ensure you're running this script from the agentic-ai directory"
        exit 1
    fi
done

echo "✅ System checks passed"
echo ""

# Create volumes with error handling
echo "📁 Creating AI volumes..."
AI_VOLUMES=("ai-cache:1GB" "ai-outputs:2GB" "ai-metrics:512MB" "ai-models:4GB")

for volume_spec in "${AI_VOLUMES[@]}"; do
    IFS=':' read -r volume_name volume_size <<< "$volume_spec"
    if ! rnx volume create "$volume_name" --size="$volume_size" --type=filesystem 2>/dev/null; then
        echo "ℹ️  Volume '$volume_name' already exists or creation failed"
        if ! rnx volume list | grep -q "$volume_name"; then
            echo "❌ Error: Failed to create $volume_name volume"
            echo "Please check available disk space (AI workloads require significant storage)"
            exit 1
        fi
    fi
done

echo ""
echo "🎬 Demo 1: LLM Inference Service"
echo "Running LLM inference with caching and metrics..."

# Check Python AI environment
echo "🔍 Checking Python AI environment..."
if ! rnx run --max-memory=512 python3 -c "import numpy, json, hashlib; print('✅ Basic Python packages available')"; then
    echo "❌ Error: Basic Python packages not available in Joblet environment"
    echo "Please ensure Python 3.8+ with standard library is available"
    exit 1
fi

if ! rnx run --upload=llm_inference.py \
       --volume=ai-cache \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=2048 \
       --max-cpu=100 \
       --env=DEMO_MODE=true \
       python3 llm_inference.py; then
    echo "❌ Error: LLM inference demo failed"
    echo "This may be due to:"
    echo "  - Insufficient memory (requires 2GB)"
    echo "  - Missing Python packages"
    echo "  - Volume mount issues"
    exit 1
fi

echo ""
echo "🤖 Demo 2: Multi-Agent Coordination System"  
echo "Running coordinated multi-agent workflow..."
if ! rnx run --upload=multi_agent_system.py \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=4096 \
       --max-cpu=200 \
       --env=AGENT_COUNT=5 \
       python3 multi_agent_system.py; then
    echo "❌ Error: Multi-agent system demo failed"
    echo "This may be due to:"
    echo "  - Insufficient memory (requires 4GB)"
    echo "  - CPU resource constraints"
    echo "  - Concurrent job limits"
    exit 1
fi

echo ""
echo "🔍 Demo 3: RAG (Retrieval-Augmented Generation) System"
echo "Running semantic search with vector database..."
if ! rnx run --upload=rag_system.py \
       --volume=ai-cache \
       --volume=ai-outputs \
       --volume=ai-metrics \
       --max-memory=3072 \
       --max-cpu=150 \
       --env=KNOWLEDGE_BASE_SIZE=large \
       python3 rag_system.py; then
    echo "❌ Error: RAG system demo failed"
    echo "This may be due to:"
    echo "  - Insufficient memory (requires 3GB)"
    echo "  - Vector processing limitations"
    exit 1
fi

echo ""
echo "⚡ Demo 4: Distributed AI Training Simulation"
echo "Simulating distributed model training workflow..."
rnx run --upload=distributed_training.py \
       --volume=ai-models \
       --volume=ai-metrics \
       --max-memory=6144 \
       --max-cpu=300 \
       --env=TRAINING_MODE=distributed \
       --env=NUM_WORKERS=4 \
       python3 distributed_training.py &

TRAINING_JOB_ID=$!
echo "Distributed training started with job ID: $TRAINING_JOB_ID"

echo ""
echo "🔄 Demo 5: AI Pipeline Orchestration"
echo "Running end-to-end AI pipeline with multiple stages..."

# Stage 1: Data Preparation
echo "Stage 1: Data Preparation"
rnx run --volume=ai-outputs \
       python3 -c "
import json
import os
import numpy as np
from datetime import datetime

# Generate sample training data
print('Generating training dataset...')
data = {
    'train_data': np.random.randn(1000, 10).tolist(),
    'train_labels': np.random.randint(0, 2, 1000).tolist(),
    'validation_data': np.random.randn(200, 10).tolist(),
    'validation_labels': np.random.randint(0, 2, 200).tolist(),
    'metadata': {
        'created_at': datetime.now().isoformat(),
        'features': 10,
        'samples': 1000,
        'classes': 2
    }
}

os.makedirs('/volumes/ai-outputs/pipeline', exist_ok=True)
with open('/volumes/ai-outputs/pipeline/training_data.json', 'w') as f:
    json.dump(data, f)

print('Training data prepared and saved')
"

# Stage 2: Model Training
echo "Stage 2: Model Training"
rnx run --volume=ai-outputs \
       --volume=ai-models \
       --max-memory=2048 \
       python3 -c "
import json
import os
import time
from datetime import datetime

print('Loading training data...')
with open('/volumes/ai-outputs/pipeline/training_data.json', 'r') as f:
    data = json.load(f)

print('Training model...')
time.sleep(3)  # Simulate training time

# Simulate model artifacts
model_info = {
    'model_id': f'model_{int(time.time())}',
    'architecture': 'neural_network',
    'parameters': 15420,
    'training_accuracy': 0.94,
    'validation_accuracy': 0.91,
    'training_time': 3.2,
    'created_at': datetime.now().isoformat()
}

os.makedirs('/volumes/ai-models/pipeline', exist_ok=True)
with open('/volumes/ai-models/pipeline/model_info.json', 'w') as f:
    json.dump(model_info, f, indent=2)

print('Model training completed')
print(f'Training Accuracy: {model_info[\"training_accuracy\"]:.2%}')
print(f'Validation Accuracy: {model_info[\"validation_accuracy\"]:.2%}')
"

# Stage 3: Model Evaluation
echo "Stage 3: Model Evaluation"
rnx run --volume=ai-outputs \
       --volume=ai-models \
       --volume=ai-metrics \
       python3 -c "
import json
import os
import time
from datetime import datetime

print('Loading model for evaluation...')
with open('/volumes/ai-models/pipeline/model_info.json', 'r') as f:
    model_info = json.load(f)

print('Running model evaluation...')
time.sleep(2)  # Simulate evaluation time

# Generate evaluation metrics
evaluation_results = {
    'model_id': model_info['model_id'],
    'test_accuracy': 0.89,
    'precision': 0.87,
    'recall': 0.91,
    'f1_score': 0.89,
    'confusion_matrix': [[45, 5], [3, 47]],
    'evaluation_time': 2.1,
    'evaluated_at': datetime.now().isoformat()
}

os.makedirs('/volumes/ai-metrics/pipeline', exist_ok=True)
with open('/volumes/ai-metrics/pipeline/evaluation_results.json', 'w') as f:
    json.dump(evaluation_results, f, indent=2)

print('Model evaluation completed')
print(f'Test Accuracy: {evaluation_results[\"test_accuracy\"]:.2%}')
print(f'F1 Score: {evaluation_results[\"f1_score\"]:.2%}')
"

# Let distributed training run a bit longer
echo "Waiting for distributed training to complete..."
sleep 10

# Stop distributed training if still running
echo "Checking distributed training status..."
rnx stop $TRAINING_JOB_ID || echo "Training job may have already completed"

echo ""
echo "✅ Agentic AI Demo Complete!"
echo ""
echo "📋 Check results:"
echo "  rnx run --volume=ai-outputs ls -la /volumes/ai-outputs/"
echo "  rnx run --volume=ai-metrics ls -la /volumes/ai-metrics/" 
echo "  rnx run --volume=ai-models ls -la /volumes/ai-models/"
echo ""
echo "📊 View demo results:"
echo "  rnx run --volume=ai-outputs find /volumes/ai-outputs -name '*.json' -exec basename {} \;"
echo "  rnx run --volume=ai-metrics cat /volumes/ai-metrics/rag_metrics_*.json | head -20"
echo ""
echo "🔍 Inspect specific outputs:"
echo "  rnx run --volume=ai-outputs cat /volumes/ai-outputs/inference_results_*.json"
echo "  rnx run --volume=ai-outputs cat /volumes/ai-outputs/workflow_results_*.json"