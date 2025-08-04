#!/bin/bash
set -e

echo "🔗 Advanced Joblet: Multi-Job Coordination"
echo "=========================================="
echo ""
echo "This demo shows advanced patterns for coordinating multiple jobs with"
echo "dependencies, data flow, and complex orchestration."
echo ""

# Check prerequisites
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    exit 1
fi

if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    exit 1
fi

echo "✅ Prerequisites checked"
echo ""

# Cleanup any existing demo resources
echo "🧹 Cleaning up previous demo resources..."
rnx volume remove pipeline-data 2>/dev/null || true
rnx volume remove stage1-output 2>/dev/null || true
rnx volume remove stage2-output 2>/dev/null || true
rnx volume remove final-results 2>/dev/null || true
rnx volume remove coordination-logs 2>/dev/null || true
echo ""

# Create volumes for job coordination
echo "📦 Creating volumes for job coordination..."
rnx volume create pipeline-data --size=200MB --type=filesystem
rnx volume create stage1-output --size=100MB --type=filesystem
rnx volume create stage2-output --size=100MB --type=filesystem
rnx volume create final-results --size=150MB --type=filesystem
rnx volume create coordination-logs --size=50MB --type=filesystem
echo "✅ Volumes created for pipeline coordination"
echo ""

echo "📋 Demo 1: Sequential Job Pipeline"
echo "----------------------------------"
echo "Running jobs in sequence with dependency management"

# Create pipeline configuration
cat > /tmp/pipeline_config.json << 'EOF'
{
  "pipeline": {
    "name": "data_processing_pipeline",
    "stages": [
      {
        "name": "data_preparation",
        "timeout": 60,
        "resources": {"cpu": 25, "memory": 256}
      },
      {
        "name": "data_analysis", 
        "timeout": 120,
        "resources": {"cpu": 50, "memory": 512}
      },
      {
        "name": "report_generation",
        "timeout": 90,
        "resources": {"cpu": 25, "memory": 256}
      }
    ]
  }
}
EOF

echo "Stage 1: Data Preparation"
STAGE1_JOB=$(rnx run --volume=pipeline-data --volume=stage1-output --volume=coordination-logs \
    --max-cpu=25 --max-memory=256 bash -c "
echo '[$(date)] Stage 1: Data Preparation Started' | tee -a /volumes/coordination-logs/pipeline.log
echo 'Preparing sample dataset...'

# Generate sample data
{
    echo 'timestamp,user_id,action,value'
    for i in {1..1000}; do
        timestamp=\$(date -d \"-\$((RANDOM % 3600)) seconds\" '+%Y-%m-%d %H:%M:%S')
        user_id=\$((RANDOM % 100 + 1))
        actions=('login' 'view_page' 'purchase' 'logout')
        action=\${actions[\$((RANDOM % 4))]}
        value=\$((RANDOM % 1000 + 1))
        echo \"\$timestamp,\$user_id,\$action,\$value\"
    done
} > /volumes/pipeline-data/raw_data.csv

echo 'Sample data generated: $(wc -l < /volumes/pipeline-data/raw_data.csv) records'

# Data validation and cleaning
echo 'Validating and cleaning data...'
grep -v '^timestamp' /volumes/pipeline-data/raw_data.csv | \
    awk -F',' '\$4 > 0 {print}' > /volumes/stage1-output/clean_data.csv

echo 'Clean data prepared: $(wc -l < /volumes/stage1-output/clean_data.csv) records'
echo 'Stage 1 output saved to: /volumes/stage1-output/clean_data.csv'

echo '[$(date)] Stage 1: Data Preparation Completed' | tee -a /volumes/coordination-logs/pipeline.log
echo 'STAGE1_SUCCESS' > /volumes/coordination-logs/stage1_status.txt
" 2>&1)

echo "✅ Stage 1 job submitted"
echo ""

# Wait for Stage 1 completion with monitoring
echo "⏳ Monitoring Stage 1 progress..."
while true; do
    if [ -f /tmp/stage1_complete ] || rnx list | grep -q "COMPLETED.*$STAGE1_JOB" 2>/dev/null; then
        break
    fi
    echo "   Stage 1 still running..."
    sleep 5
done

# Verify Stage 1 completion
sleep 3
echo "Stage 2: Data Analysis (depends on Stage 1)"
STAGE2_JOB=$(rnx run --volume=stage1-output --volume=stage2-output --volume=coordination-logs \
    --max-cpu=50 --max-memory=512 bash -c "
echo '[$(date)] Stage 2: Data Analysis Started' | tee -a /volumes/coordination-logs/pipeline.log

# Check dependency
if [ ! -f /volumes/coordination-logs/stage1_status.txt ]; then
    echo 'ERROR: Stage 1 dependency not met'
    exit 1
fi

stage1_status=\$(cat /volumes/coordination-logs/stage1_status.txt)
if [ \"\$stage1_status\" != 'STAGE1_SUCCESS' ]; then
    echo 'ERROR: Stage 1 did not complete successfully'
    exit 1
fi

echo 'Stage 1 dependency verified successfully'
echo 'Starting analysis of clean data...'

# Analyze the data
input_file='/volumes/stage1-output/clean_data.csv'
if [ -f \"\$input_file\" ]; then
    echo 'Input data found: \$(wc -l < \$input_file) records'
    
    # User activity analysis
    echo 'Performing user activity analysis...'
    {
        echo 'user_id,login_count,page_views,purchases,total_value'
        awk -F',' '
        {
            user=\$2; action=\$3; value=\$4
            users[user][action]++
            if(action==\"purchase\") users[user][\"total_value\"]+=value
        }
        END {
            for(user in users) {
                printf \"%s,%d,%d,%d,%d\\n\", user, 
                    users[user][\"login\"], users[user][\"view_page\"], 
                    users[user][\"purchase\"], users[user][\"total_value\"]
            }
        }' \"\$input_file\"
    } > /volumes/stage2-output/user_analysis.csv
    
    # Summary statistics
    echo 'Generating summary statistics...'
    {
        echo 'Analysis Summary'
        echo '==============='
        echo 'Total records: '\$(wc -l < \$input_file)
        echo 'Unique users: '\$(awk -F',' '{print \$2}' \$input_file | sort -u | wc -l)
        echo 'Total purchases: '\$(grep 'purchase' \$input_file | wc -l)
        echo 'Total revenue: '\$(grep 'purchase' \$input_file | awk -F',' '{sum+=\$4} END {print sum}')
    } > /volumes/stage2-output/summary.txt
    
    echo 'Analysis completed successfully'
    echo 'Results saved to:'
    echo '  - User analysis: /volumes/stage2-output/user_analysis.csv'
    echo '  - Summary: /volumes/stage2-output/summary.txt'
else
    echo 'ERROR: Input file not found'
    exit 1
fi

echo '[$(date)] Stage 2: Data Analysis Completed' | tee -a /volumes/coordination-logs/pipeline.log
echo 'STAGE2_SUCCESS' > /volumes/coordination-logs/stage2_status.txt
")

echo "✅ Stage 2 job submitted"
echo ""

# Wait for Stage 2
echo "⏳ Monitoring Stage 2 progress..."
sleep 8

echo "Stage 3: Report Generation (depends on Stage 2)"
STAGE3_JOB=$(rnx run --volume=stage2-output --volume=final-results --volume=coordination-logs \
    --max-cpu=25 --max-memory=256 bash -c "
echo '[$(date)] Stage 3: Report Generation Started' | tee -a /volumes/coordination-logs/pipeline.log

# Check dependencies
if [ ! -f /volumes/coordination-logs/stage2_status.txt ]; then
    echo 'ERROR: Stage 2 dependency not met'
    exit 1
fi

stage2_status=\$(cat /volumes/coordination-logs/stage2_status.txt)
if [ \"\$stage2_status\" != 'STAGE2_SUCCESS' ]; then
    echo 'ERROR: Stage 2 did not complete successfully'
    exit 1
fi

echo 'Stage 2 dependency verified successfully'
echo 'Generating final report...'

# Generate comprehensive report
{
    echo '# Data Processing Pipeline Report'
    echo '=================================='
    echo ''
    echo 'Generated on: '\$(date)
    echo ''
    echo '## Summary Statistics'
    cat /volumes/stage2-output/summary.txt
    echo ''
    echo '## Top 10 Users by Purchase Value'
    echo 'user_id,purchases,total_value'
    head -1 /volumes/stage2-output/user_analysis.csv > /dev/null
    tail -n +2 /volumes/stage2-output/user_analysis.csv | \
        sort -t',' -k5 -nr | head -10 | \
        awk -F',' '{printf \"%s,%s,%s\\n\", \$1, \$4, \$5}'
    echo ''
    echo '## Processing Log'
    cat /volumes/coordination-logs/pipeline.log
} > /volumes/final-results/final_report.md

echo 'Final report generated: /volumes/final-results/final_report.md'

# Create JSON summary for API consumption
{
    echo '{'
    echo '  \"pipeline\": \"data_processing_pipeline\",'
    echo '  \"completed_at\": \"'\$(date -Iseconds)'\",'
    echo '  \"status\": \"SUCCESS\",'
    echo '  \"stages_completed\": 3,'
    echo '  \"output_files\": ['
    echo '    \"/volumes/final-results/final_report.md\"'
    echo '  ]'
    echo '}'
} > /volumes/final-results/pipeline_result.json

echo '[$(date)] Stage 3: Report Generation Completed' | tee -a /volumes/coordination-logs/pipeline.log
echo 'PIPELINE_SUCCESS' > /volumes/coordination-logs/pipeline_status.txt
echo 'Sequential pipeline completed successfully!'
")

echo "✅ Stage 3 job submitted"
echo ""

# Wait for pipeline completion
echo "⏳ Waiting for complete pipeline..."
sleep 10

echo "📋 Demo 2: Parallel Job Execution with Aggregation"
echo "--------------------------------------------------"
echo "Running multiple jobs in parallel and aggregating results"

# Clean up previous parallel demo
rnx volume remove parallel-shard-1 2>/dev/null || true
rnx volume remove parallel-shard-2 2>/dev/null || true
rnx volume remove parallel-shard-3 2>/dev/null || true
rnx volume remove parallel-results 2>/dev/null || true

# Create shard volumes
rnx volume create parallel-shard-1 --size=50MB --type=memory
rnx volume create parallel-shard-2 --size=50MB --type=memory  
rnx volume create parallel-shard-3 --size=50MB --type=memory
rnx volume create parallel-results --size=100MB --type=filesystem

echo "Starting parallel processing jobs..."

# Start parallel jobs
PARALLEL_JOB1=$(rnx run --volume=parallel-shard-1 --max-cpu=33 --max-memory=256 bash -c "
echo 'Parallel Job 1: Processing shard 1'
# Generate shard 1 data
for i in {1..100}; do
    echo \"\$i,shard1,\$((RANDOM % 1000))\" >> /volumes/parallel-shard-1/data.csv
done
# Process shard 1
sum=\$(awk -F',' '{sum += \$3} END {print sum}' /volumes/parallel-shard-1/data.csv)
echo \"Shard 1 processing complete: sum=\$sum\"
echo \"shard1,\$sum\" > /volumes/parallel-shard-1/result.csv
")

PARALLEL_JOB2=$(rnx run --volume=parallel-shard-2 --max-cpu=33 --max-memory=256 bash -c "
echo 'Parallel Job 2: Processing shard 2'
# Generate shard 2 data
for i in {101..200}; do
    echo \"\$i,shard2,\$((RANDOM % 1000))\" >> /volumes/parallel-shard-2/data.csv
done
# Process shard 2
sum=\$(awk -F',' '{sum += \$3} END {print sum}' /volumes/parallel-shard-2/data.csv)
echo \"Shard 2 processing complete: sum=\$sum\"
echo \"shard2,\$sum\" > /volumes/parallel-shard-2/result.csv
")

PARALLEL_JOB3=$(rnx run --volume=parallel-shard-3 --max-cpu=33 --max-memory=256 bash -c "
echo 'Parallel Job 3: Processing shard 3'
# Generate shard 3 data
for i in {201..300}; do
    echo \"\$i,shard3,\$((RANDOM % 1000))\" >> /volumes/parallel-shard-3/data.csv
done
# Process shard 3
sum=\$(awk -F',' '{sum += \$3} END {print sum}' /volumes/parallel-shard-3/data.csv)
echo \"Shard 3 processing complete: sum=\$sum\"
echo \"shard3,\$sum\" > /volumes/parallel-shard-3/result.csv
")

echo "✅ Parallel jobs started:"
echo "   Job 1: $PARALLEL_JOB1"
echo "   Job 2: $PARALLEL_JOB2" 
echo "   Job 3: $PARALLEL_JOB3"
echo ""

# Wait for parallel jobs to complete
echo "⏳ Waiting for parallel jobs to complete..."
sleep 8

# Aggregation job
echo "Starting result aggregation job..."
AGGREGATION_JOB=$(rnx run --volume=parallel-shard-1 --volume=parallel-shard-2 --volume=parallel-shard-3 --volume=parallel-results bash -c "
echo 'Aggregation Job: Combining results from parallel processing'

# Wait for all shard results
echo 'Checking for shard results...'
for shard in 1 2 3; do
    result_file=\"/volumes/parallel-shard-\$shard/result.csv\"
    if [ -f \"\$result_file\" ]; then
        echo \"Shard \$shard result found: \$(cat \$result_file)\"
    else
        echo \"Warning: Shard \$shard result not found\"
    fi
done

# Aggregate results
echo 'Aggregating results...'
{
    echo 'shard,sum'
    cat /volumes/parallel-shard-1/result.csv 2>/dev/null || echo 'shard1,0'
    cat /volumes/parallel-shard-2/result.csv 2>/dev/null || echo 'shard2,0' 
    cat /volumes/parallel-shard-3/result.csv 2>/dev/null || echo 'shard3,0'
} > /volumes/parallel-results/aggregated.csv

# Calculate total
total=\$(tail -n +2 /volumes/parallel-results/aggregated.csv | awk -F',' '{sum += \$2} END {print sum}')
echo \"Total across all shards: \$total\"

# Generate summary
{
    echo '# Parallel Processing Results'
    echo '============================'
    echo ''
    echo 'Processing completed at: '\$(date)
    echo 'Number of shards: 3'
    echo 'Total sum: '\$total
    echo ''
    echo '## Individual Shard Results'
    cat /volumes/parallel-results/aggregated.csv
} > /volumes/parallel-results/summary.md

echo 'Parallel processing and aggregation completed successfully!'
echo 'Results saved to: /volumes/parallel-results/'
")

echo "✅ Aggregation job started"
echo ""

# Wait for aggregation
sleep 6

echo "📋 Demo 3: Dynamic Job Scheduling"
echo "---------------------------------"
echo "Demonstrating conditional job execution based on results"

# Create monitoring volume
rnx volume remove dynamic-control 2>/dev/null || true
rnx volume create dynamic-control --size=50MB --type=memory

echo "Starting controller job for dynamic scheduling..."
CONTROLLER_JOB=$(rnx run --volume=dynamic-control --volume=final-results bash -c "
echo 'Dynamic Controller: Analyzing pipeline results for conditional execution'

# Check pipeline results
if [ -f /volumes/final-results/pipeline_result.json ]; then
    echo 'Pipeline results found, analyzing...'
    
    # Extract key metrics (simulated)
    success_rate=85  # Simulated metric
    data_quality=92  # Simulated metric
    
    echo \"Success rate: \$success_rate%\"
    echo \"Data quality: \$data_quality%\"
    
    # Decision logic
    if [ \$success_rate -gt 80 ] && [ \$data_quality -gt 90 ]; then
        echo 'DECISION: High quality results - triggering advanced analysis'
        echo 'trigger_advanced_analysis' > /volumes/dynamic-control/next_action.txt
    elif [ \$success_rate -gt 60 ]; then
        echo 'DECISION: Moderate quality - triggering standard cleanup'
        echo 'trigger_cleanup' > /volumes/dynamic-control/next_action.txt
    else
        echo 'DECISION: Low quality - triggering reprocessing'
        echo 'trigger_reprocess' > /volumes/dynamic-control/next_action.txt
    fi
    
    # Save decision context
    {
        echo 'success_rate='\$success_rate
        echo 'data_quality='\$data_quality
        echo 'decision_time='\$(date -Iseconds)
    } > /volumes/dynamic-control/decision_context.txt
    
else
    echo 'No pipeline results found - triggering fallback'
    echo 'trigger_fallback' > /volumes/dynamic-control/next_action.txt
fi

echo 'Dynamic controller completed - decision recorded'
")

sleep 5

# Execute conditional job based on controller decision
CONDITIONAL_JOB=$(rnx run --volume=dynamic-control --volume=final-results bash -c "
echo 'Conditional Execution: Reading controller decision'

if [ -f /volumes/dynamic-control/next_action.txt ]; then
    action=\$(cat /volumes/dynamic-control/next_action.txt)
    echo \"Controller decision: \$action\"
    
    case \$action in
        'trigger_advanced_analysis')
            echo 'Executing advanced analysis workflow...'
            echo 'Advanced analysis completed' > /volumes/final-results/advanced_analysis.txt
            ;;
        'trigger_cleanup')
            echo 'Executing standard cleanup workflow...'
            echo 'Standard cleanup completed' > /volumes/final-results/cleanup.txt
            ;;
        'trigger_reprocess')
            echo 'Executing reprocessing workflow...'
            echo 'Reprocessing completed' > /volumes/final-results/reprocess.txt
            ;;
        'trigger_fallback')
            echo 'Executing fallback workflow...'
            echo 'Fallback completed' > /volumes/final-results/fallback.txt
            ;;
        *)
            echo 'Unknown action - executing default workflow'
            echo 'Default workflow completed' > /volumes/final-results/default.txt
            ;;
    esac
    
    echo 'Conditional execution completed successfully'
else
    echo 'No controller decision found - skipping conditional execution'
fi
")

echo "✅ Dynamic scheduling jobs completed"
echo ""

sleep 3

echo "📋 Demo 4: Job Coordination Results"
echo "-----------------------------------"
echo "Reviewing results from coordinated job execution"

# Display pipeline results
echo "Sequential Pipeline Results:"
rnx run --volume=final-results bash -c "
if [ -f /volumes/final-results/final_report.md ]; then
    echo 'Pipeline report generated successfully:'
    head -20 /volumes/final-results/final_report.md
else
    echo 'Pipeline report not found'
fi
"

echo ""
echo "Parallel Processing Results:"
rnx run --volume=parallel-results bash -c "
if [ -f /volumes/parallel-results/summary.md ]; then
    echo 'Parallel processing summary:'
    cat /volumes/parallel-results/summary.md
else
    echo 'Parallel processing results not found'
fi
"

echo ""
echo "Dynamic Scheduling Results:"
rnx run --volume=dynamic-control --volume=final-results bash -c "
echo 'Controller decision:'
if [ -f /volumes/dynamic-control/decision_context.txt ]; then
    cat /volumes/dynamic-control/decision_context.txt
    echo ''
    
    action=\$(cat /volumes/dynamic-control/next_action.txt 2>/dev/null || echo 'none')
    echo \"Action taken: \$action\"
    
    # Show execution result
    case \$action in
        'trigger_advanced_analysis')
            [ -f /volumes/final-results/advanced_analysis.txt ] && cat /volumes/final-results/advanced_analysis.txt
            ;;
        'trigger_cleanup')
            [ -f /volumes/final-results/cleanup.txt ] && cat /volumes/final-results/cleanup.txt
            ;;
        *)
            echo 'Other action executed'
            ;;
    esac
else
    echo 'No decision context found'
fi
"

echo ""
echo "✅ Multi-Job Coordination Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • Sequential job pipelines with dependency management"
echo "  • Parallel job execution with result aggregation"
echo "  • Dynamic job scheduling based on conditional logic"
echo "  • Data flow coordination between jobs using volumes"
echo "  • Error handling and pipeline recovery patterns"
echo "  • Resource optimization for coordinated workloads"
echo ""
echo "📝 Key coordination patterns:"
echo "  • Stage-gate pattern for sequential workflows"
echo "  • Fork-join pattern for parallel processing"
echo "  • Controller pattern for dynamic scheduling"
echo "  • Volume-based data sharing between jobs"
echo "  • Status files for dependency tracking"
echo ""
echo "💡 Production considerations:"
echo "  • Implement timeout handling for job dependencies"
echo "  • Use exponential backoff for job coordination"
echo "  • Monitor resource usage across coordinated jobs"
echo "  • Implement cleanup procedures for failed pipelines"
echo "  • Use persistent volumes for critical coordination data"
echo ""
echo "➡️  Next: Try ./02_distributed_processing.sh for large-scale patterns"