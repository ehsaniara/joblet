# Data Pipeline Workflow

ETL (Extract, Transform, Load) pipeline demonstrating data processing with shared volumes.

## Files

- `data-pipeline.yaml` - Workflow definition
- `extract.py` - Data extraction script
- `validate.py` - Data validation script
- `transform.py` - Data transformation script
- `load.py` - Data loading script
- `report.py` - Report generation script

## Usage

```bash
# From joblet root directory  
cd examples/workflows/data-pipeline

# Run individual jobs
rnx job run --workflow=data-pipeline.yaml:extract-data
rnx job run --workflow=data-pipeline.yaml:validate-data

# Attempt full workflow (pending integration)
rnx job run --workflow=data-pipeline.yaml
```

## Data Flow

1. **extract.py** → `data/raw_data.json`
2. **validate.py** → `data/validated_data.json`
3. **transform.py** → `data/transformed_data.json`
4. **load.py** → `data/warehouse_data.json`
5. **report.py** → `data/pipeline_report.md`

All data files are stored in the shared `data-pipeline` volume.