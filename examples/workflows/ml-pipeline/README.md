# ML Pipeline Workflow

Complete machine learning pipeline demonstrating sequential job dependencies with real Python scripts.

## Files

- `ml-pipeline.yaml` - Workflow definition with Python 3.11 runtime
- `prepare_data.py` - Data preparation script
- `select_features.py` - Feature selection script
- `train.py` - Model training script
- `evaluate.py` - Model evaluation script
- `test_model.py` - Deployment testing script

## Usage

```bash
# From joblet root directory
cd examples/workflows/ml-pipeline

# Run individual jobs
rnx workflow run ml-pipeline.yaml
rnx workflow run ml-pipeline.yaml

# Attempt full workflow (pending integration)
rnx workflow run ml-pipeline.yaml
```

## Data Flow

1. **prepare_data.py** → `ml_data/prepared_dataset.json`
2. **select_features.py** → `ml_data/selected_features.json`
3. **train.py** → `ml_data/trained_model.json`
4. **evaluate.py** → `ml_data/evaluation_results.json`
5. **test_model.py** → `deployment_test_results.json`

All data files are stored in the shared `ml-pipeline` volume.