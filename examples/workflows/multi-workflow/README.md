# Multi-Workflow Example

Demonstrates multiple named workflows in a single YAML file.

## Files

- `multi-workflow.yaml` - Contains two named workflows
- `prepare_data.py` - ML data preparation script
- `select_features.py` - ML feature selection script
- `train.py` - ML model training script
- `evaluate.py` - ML model evaluation script
- `test_model.py` - Deployment testing script

## Workflows

### 1. ml-training

Complete ML training pipeline:

- data-prep → feature-selection → train-model → evaluate-model

### 2. deployment

Model deployment pipeline:

- package-model → test-model → deploy-staging → deploy-production

## Usage

```bash
# From joblet root directory
cd examples/workflows/multi-workflow

# Run specific workflow (pending integration)
rnx job run --workflow=multi-workflow.yaml:ml-training
rnx job run --workflow=multi-workflow.yaml:deployment

# Note: Individual job selection doesn't work with multi-workflow format
# Jobs are nested under workflows, not at the top level
# Use individual workflow folders instead for single job testing
```

## Data Flow

Same as ml-pipeline workflow - all ML data stored in shared `ml-pipeline` volume.