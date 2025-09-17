# ETL Processing Workflow Examples

This directory contains examples for Extract, Transform, Load (ETL) data processing workflows with dependencies.

## Files

### `etl-pipeline.yaml`

Contains a complete ETL pipeline with job dependencies:

1. **etl** - Multi-step ETL process (extract → transform → load)
2. **validate** - Data quality validation (depends on etl)
3. **compress** - Archive old data (depends on validate)
4. **db-backup** - Database backup (independent)
5. **report** - Generate reports (depends on compress)
6. **sync** - Data synchronization (independent)
7. **cleanup** - Clean up temporary files (depends on report)

## Workflow Dependencies

```
etl → validate → compress → report → cleanup
      ↓                    ↑
   db-backup            sync (independent)
```

## Usage Examples

```bash
# Run the complete ETL pipeline
rnx job run --workflow=etl-pipeline.yaml

# Run individual jobs
rnx job run --workflow=etl-pipeline.yaml:etl
rnx job run --workflow=etl-pipeline.yaml:validate
rnx job run --workflow=etl-pipeline.yaml:db-backup
```

## Prerequisites

### Required Volumes

```bash
rnx volume create raw-data
rnx volume create processed-data
rnx volume create etl-logs
rnx volume create validation-reports
rnx volume create backups
rnx volume create reports
rnx volume create source-data
rnx volume create backup-data
rnx volume create logs
```

### Required Files

Create these Python scripts in a `scripts/` directory:

- `extract.py` - Data extraction script
- `transform.py` - Data transformation script
- `load.py` - Data loading script
- `validate.py` - Data validation script
- `validation_rules.yaml` - Validation configuration
- `generate_report.py` - Report generation script
- `report_template.html` - Report template

## Features Demonstrated

- **Job Dependencies**: Sequential workflow execution
- **Resource Limits**: Memory, CPU, and I/O bandwidth controls
- **Volume Management**: Persistent data storage across jobs
- **Error Handling**: Robust pipeline with validation steps
- **Data Archival**: Compression and cleanup processes
- **Reporting**: Automated report generation