# Parallel Jobs Workflow

Demonstrates parallel batch processing without dependencies between jobs.

## Files

- `parallel-jobs.yaml` - Workflow definition
- `process_batch1.py` - Processes batch 1 (3 items, 2s processing time)
- `process_batch2.py` - Processes batch 2 (2 items, 1.5s processing time)
- `process_batch3.py` - Processes batch 3 (4 items, 3s processing time)

## Usage

```bash
# From joblet root directory
cd examples/workflows/parallel-jobs

# Run individual batches
rnx job run --workflow=parallel-jobs.yaml:batch1
rnx job run --workflow=parallel-jobs.yaml:batch2
rnx job run --workflow=parallel-jobs.yaml:batch3

# Run all batches in parallel (pending integration)
rnx job run --workflow=parallel-jobs.yaml
```

## Output

Each batch script creates:

- `results/batch1_results.json`
- `results/batch2_results.json`
- `results/batch3_results.json`

All results are stored in the shared `parallel-processing` volume.