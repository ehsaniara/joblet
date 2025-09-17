# Basic Usage Workflow Examples

This directory contains simple workflow examples demonstrating basic Joblet functionality.

## Files

### `basic-jobs.yaml`

Contains four basic job examples:

1. **hello** - Simple echo command demonstration
2. **analytics** - Python data analysis with file uploads and volumes
3. **webserver** - Long-running nginx server example
4. **backup** - Scheduled backup task with volume operations

### Supporting Files

- `analyze.py` - Python script for the analytics job
- `data.csv` - Sample data file for analysis

## Usage Examples

```bash
# Run individual jobs
rnx job run --workflow=basic-jobs.yaml:hello
rnx job run --workflow=basic-jobs.yaml:analytics
rnx job run --workflow=basic-jobs.yaml:webserver
rnx job run --workflow=basic-jobs.yaml:backup

# Run the entire workflow (all jobs)
rnx job run --workflow=basic-jobs.yaml
```

## Prerequisites

Ensure these volumes exist before running:

- `analytics-data` - For analytics job data storage
- `web-content` - For webserver content
- `backups` - For backup storage
- `data` - For source data to backup

Create volumes with:

```bash
rnx volume create analytics-data
rnx volume create web-content
rnx volume create backups
rnx volume create data
```