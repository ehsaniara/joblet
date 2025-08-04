#!/bin/bash
set -e

echo "🌐 Advanced Joblet: Distributed Processing"
echo "==========================================="
echo ""
echo "This demo shows large-scale distributed processing patterns using"
echo "MapReduce, data sharding, and fault-tolerant parallel processing."
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

# Cleanup previous distributed processing resources
echo "🧹 Cleaning up previous distributed processing resources..."
for i in {1..8}; do
    rnx volume remove map-input-$i 2>/dev/null || true
    rnx volume remove map-output-$i 2>/dev/null || true
    rnx volume remove reduce-input-$i 2>/dev/null || true
done
rnx volume remove distributed-data 2>/dev/null || true
rnx volume remove reduce-output 2>/dev/null || true
rnx volume remove mapreduce-control 2>/dev/null || true
rnx volume remove final-output 2>/dev/null || true
echo ""

echo "📦 Creating volumes for distributed processing..."
rnx volume create distributed-data --size=500MB --type=filesystem
rnx volume create mapreduce-control --size=100MB --type=filesystem
rnx volume create final-output --size=200MB --type=filesystem

# Create map and reduce volumes
for i in {1..8}; do
    rnx volume create map-input-$i --size=64MB --type=memory
    rnx volume create map-output-$i --size=64MB --type=memory
done

for i in {1..4}; do
    rnx volume create reduce-input-$i --size=128MB --type=memory
done

rnx volume create reduce-output --size=200MB --type=filesystem
echo "✅ Distributed processing volumes created"
echo ""

echo "📋 Demo 1: Data Partitioning Phase"
echo "----------------------------------"
echo "Creating and partitioning large dataset for distributed processing"

DATA_PREP_JOB=$(rnx run --volume=distributed-data --volume=mapreduce-control --max-memory=512 bash -c "
echo 'Data Preparation: Creating large dataset for distributed processing'

# Generate large sample dataset (simulating log analysis)
echo 'Generating large sample dataset...'
{
    echo 'timestamp,ip_address,method,url,status_code,response_size,user_agent'
    for i in {1..10000}; do
        timestamp=\$(date -d \"-\$((RANDOM % 86400)) seconds\" '+%Y-%m-%d %H:%M:%S')
        ip=\"\$((RANDOM % 256)).\$((RANDOM % 256)).\$((RANDOM % 256)).\$((RANDOM % 256))\"
        methods=('GET' 'POST' 'PUT' 'DELETE')
        method=\${methods[\$((RANDOM % 4))]}
        
        urls=('/api/users' '/api/products' '/api/orders' '/static/js/app.js' '/static/css/style.css' '/images/logo.png')
        url=\${urls[\$((RANDOM % 6))]}
        
        statuses=(200 200 200 200 404 500 301)
        status=\${statuses[\$((RANDOM % 7))]}
        
        response_size=\$((RANDOM % 10000 + 100))
        user_agents=('Chrome/91.0' 'Firefox/89.0' 'Safari/14.1' 'Edge/91.0')
        user_agent=\${user_agents[\$((RANDOM % 4))]}
        
        echo \"\$timestamp,\$ip,\$method,\$url,\$status,\$response_size,\$user_agent\"
    done
} > /volumes/distributed-data/access_logs.csv

echo 'Dataset created: \$(wc -l < /volumes/distributed-data/access_logs.csv) records'

# Create partitioning configuration
{
    echo '{'
    echo '  \"partitioning\": {'
    echo '    \"strategy\": \"hash\",'
    echo '    \"partitions\": 8,'
    echo '    \"field\": \"ip_address\"'
    echo '  },'
    echo '  \"processing\": {'
    echo '    \"map_workers\": 8,'
    echo '    \"reduce_workers\": 4'
    echo '  }'
    echo '}'
} > /volumes/mapreduce-control/config.json

echo 'Partitioning configuration created'
echo 'Data preparation completed successfully'
")

echo "✅ Data preparation job started"
sleep 5

# Data partitioning job
PARTITION_JOB=$(rnx run --volume=distributed-data --volume=mapreduce-control \
    $(for i in {1..8}; do echo "--volume=map-input-$i"; done) bash -c "
echo 'Data Partitioning: Splitting dataset into 8 partitions'

input_file='/volumes/distributed-data/access_logs.csv'
if [ ! -f \"\$input_file\" ]; then
    echo 'ERROR: Input file not found'
    exit 1
fi

echo 'Partitioning data by IP address hash...'

# Read header
header=\$(head -1 \"\$input_file\")

# Partition data (skip header, then distribute based on IP hash)
tail -n +2 \"\$input_file\" | while IFS=',' read -r line; do
    ip=\$(echo \"\$line\" | cut -d',' -f2)
    # Simple hash function based on last octet
    partition=\$(echo \"\$ip\" | cut -d'.' -f4)
    partition=\$((partition % 8 + 1))
    
    # Write to appropriate partition
    echo \"\$line\" >> \"/volumes/map-input-\$partition/data.csv\"
done

# Add headers to all partitions
for i in {1..8}; do
    if [ -f \"/volumes/map-input-\$i/data.csv\" ]; then
        # Prepend header
        echo \"\$header\" > \"/tmp/temp_\$i.csv\"
        cat \"/volumes/map-input-\$i/data.csv\" >> \"/tmp/temp_\$i.csv\"
        mv \"/tmp/temp_\$i.csv\" \"/volumes/map-input-\$i/data.csv\"
        
        record_count=\$(wc -l < \"/volumes/map-input-\$i/data.csv\")
        echo \"Partition \$i: \$record_count records\"
    else
        echo \"Partition \$i: 0 records (empty)\"
        echo \"\$header\" > \"/volumes/map-input-\$i/data.csv\"
    fi
done

echo 'Data partitioning completed - 8 partitions created'
echo 'PARTITION_SUCCESS' > /volumes/mapreduce-control/partition_status.txt
")

echo "✅ Data partitioning job started"
sleep 8

echo "📋 Demo 2: Map Phase - Parallel Processing"
echo "------------------------------------------"
echo "Running 8 parallel map workers for distributed processing"

# Start map workers in parallel
declare -a MAP_JOBS
for i in {1..8}; do
    MAP_JOBS[$i]=$(rnx run --volume=map-input-$i --volume=map-output-$i \
        --max-cpu=25 --max-memory=256 bash -c "
    echo 'Map Worker $i: Processing partition $i'
    
    input_file='/volumes/map-input-$i/data.csv'
    
    if [ ! -f \"\$input_file\" ] || [ \$(wc -l < \"\$input_file\") -eq 1 ]; then
        echo 'No data in partition $i - creating empty output'
        echo 'status_code,count' > /volumes/map-output-$i/results.csv
        exit 0
    fi
    
    echo 'Processing \$(wc -l < \"\$input_file\") records in partition $i...'
    
    # Map operation: Count status codes
    {
        echo 'status_code,count'
        tail -n +2 \"\$input_file\" | cut -d',' -f5 | sort | uniq -c | \
            awk '{printf \"%s,%s\\n\", \$2, \$1}'
    } > /volumes/map-output-$i/results.csv
    
    # Additional analysis: response size statistics
    {
        echo 'metric,value'
        echo 'total_requests,'\$(tail -n +2 \"\$input_file\" | wc -l)
        echo 'avg_response_size,'\$(tail -n +2 \"\$input_file\" | cut -d',' -f6 | \
            awk '{sum+=\$1; count++} END {if(count>0) print int(sum/count); else print 0}')
        echo 'max_response_size,'\$(tail -n +2 \"\$input_file\" | cut -d',' -f6 | sort -n | tail -1)
    } > /volumes/map-output-$i/metrics.csv
    
    echo 'Map Worker $i completed successfully'
    ")
    
    echo "   Map Worker $i started (Job: ${MAP_JOBS[$i]})"
done

echo "✅ All 8 map workers started in parallel"
echo ""

# Monitor map phase completion
echo "⏳ Monitoring map phase progress..."
sleep 10

echo "📋 Demo 3: Shuffle Phase - Data Redistribution"
echo "----------------------------------------------"
echo "Redistributing map outputs for reduce phase"

SHUFFLE_JOB=$(rnx run $(for i in {1..8}; do echo "--volume=map-output-$i"; done) \
    $(for i in {1..4}; do echo "--volume=reduce-input-$i"; done) \
    --volume=mapreduce-control bash -c "
echo 'Shuffle Phase: Redistributing map outputs for reduce workers'

# Initialize reduce input files
for i in {1..4}; do
    echo 'status_code,count' > \"/volumes/reduce-input-\$i/status_codes.csv\"
    echo 'metric,value' > \"/volumes/reduce-input-\$i/metrics.csv\"
done

echo 'Collecting and redistributing status code counts...'

# Collect all status code results
for map_partition in {1..8}; do
    if [ -f \"/volumes/map-output-\$map_partition/results.csv\" ]; then
        echo \"Processing map output from partition \$map_partition\"
        
        # Skip header and redistribute by status code
        tail -n +2 \"/volumes/map-output-\$map_partition/results.csv\" | while IFS=',' read -r status count; do
            # Distribute status codes to reduce workers (simple hash)
            case \$status in
                200) reduce_worker=1 ;;
                404) reduce_worker=2 ;;
                500) reduce_worker=3 ;;
                *) reduce_worker=4 ;;
            esac
            
            echo \"\$status,\$count\" >> \"/volumes/reduce-input-\$reduce_worker/status_codes.csv\"
        done
        
        # Collect metrics
        if [ -f \"/volumes/map-output-\$map_partition/metrics.csv\" ]; then
            tail -n +2 \"/volumes/map-output-\$map_partition/metrics.csv\" >> \"/volumes/reduce-input-1/metrics.csv\"
        fi
    fi
done

echo 'Data redistribution completed'

# Show shuffle statistics
for i in {1..4}; do
    status_count=\$(wc -l < \"/volumes/reduce-input-\$i/status_codes.csv\")
    echo \"Reduce worker \$i: \$status_count status code entries\"
done

echo 'SHUFFLE_SUCCESS' > /volumes/mapreduce-control/shuffle_status.txt
")

echo "✅ Shuffle phase job started"
sleep 8

echo "📋 Demo 4: Reduce Phase - Result Aggregation"
echo "--------------------------------------------"
echo "Running 4 reduce workers to aggregate results"

# Start reduce workers
declare -a REDUCE_JOBS
for i in {1..4}; do
    REDUCE_JOBS[$i]=$(rnx run --volume=reduce-input-$i --volume=reduce-output \
        --max-cpu=50 --max-memory=256 bash -c "
    echo 'Reduce Worker $i: Aggregating results'
    
    # Aggregate status codes
    if [ -f '/volumes/reduce-input-$i/status_codes.csv' ]; then
        echo 'Aggregating status codes for reduce worker $i...'
        
        # Skip header and aggregate
        {
            echo 'status_code,total_count'
            tail -n +2 '/volumes/reduce-input-$i/status_codes.csv' | \
                awk -F',' '{status[\$1] += \$2} END {for(s in status) print s \",\" status[s]}' | \
                sort -t',' -k1,1n
        } > '/volumes/reduce-output/status_codes_worker_$i.csv'
        
        echo 'Status code aggregation completed for worker $i'
    fi
    
    # Special handling for metrics (only worker 1)
    if [ '$i' -eq 1 ] && [ -f '/volumes/reduce-input-$i/metrics.csv' ]; then
        echo 'Aggregating metrics...'
        
        # Calculate overall metrics
        total_requests=\$(grep 'total_requests' '/volumes/reduce-input-$i/metrics.csv' | \
            cut -d',' -f2 | awk '{sum += \$1} END {print sum}')
        
        avg_response_size=\$(grep 'avg_response_size' '/volumes/reduce-input-$i/metrics.csv' | \
            cut -d',' -f2 | awk '{sum += \$1; count++} END {if(count>0) print int(sum/count); else print 0}')
        
        max_response_size=\$(grep 'max_response_size' '/volumes/reduce-input-$i/metrics.csv' | \
            cut -d',' -f2 | sort -n | tail -1)
        
        {
            echo 'metric,value'
            echo \"total_requests,\$total_requests\"
            echo \"avg_response_size,\$avg_response_size\"
            echo \"max_response_size,\$max_response_size\"
        } > '/volumes/reduce-output/aggregated_metrics.csv'
        
        echo 'Metrics aggregation completed'
    fi
    
    echo 'Reduce Worker $i completed successfully'
    ")
    
    echo "   Reduce Worker $i started (Job: ${REDUCE_JOBS[$i]})"
done

echo "✅ All 4 reduce workers started"
sleep 8

echo "📋 Demo 5: Final Aggregation and Results"
echo "----------------------------------------"
echo "Combining reduce outputs into final results"

FINAL_AGGREGATION_JOB=$(rnx run --volume=reduce-output --volume=final-output \
    --volume=mapreduce-control bash -c "
echo 'Final Aggregation: Combining all reduce outputs'

# Combine all status code results
{
    echo 'status_code,total_count'
    for worker in {1..4}; do
        if [ -f \"/volumes/reduce-output/status_codes_worker_\$worker.csv\" ]; then
            tail -n +2 \"/volumes/reduce-output/status_codes_worker_\$worker.csv\"
        fi
    done | awk -F',' '{status[\$1] += \$2} END {for(s in status) print s \",\" status[s]}' | sort -t',' -k1,1n
} > /volumes/final-output/final_status_codes.csv

echo 'Status codes aggregated across all workers'

# Copy metrics
if [ -f '/volumes/reduce-output/aggregated_metrics.csv' ]; then
    cp '/volumes/reduce-output/aggregated_metrics.csv' '/volumes/final-output/'
    echo 'Metrics copied to final output'
fi

# Generate comprehensive report
{
    echo '# Distributed Processing Results'
    echo '==============================='
    echo ''
    echo 'Processing completed at: '\$(date)
    echo 'MapReduce job completed successfully'
    echo ''
    echo '## Processing Statistics'
    if [ -f '/volumes/final-output/aggregated_metrics.csv' ]; then
        echo 'Total requests processed: '\$(grep 'total_requests' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)
        echo 'Average response size: '\$(grep 'avg_response_size' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)' bytes'
        echo 'Maximum response size: '\$(grep 'max_response_size' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)' bytes'
    fi
    echo ''
    echo '## Status Code Distribution'
    cat /volumes/final-output/final_status_codes.csv
    echo ''
    echo '## Processing Architecture'
    echo '- Map workers: 8 parallel processors'
    echo '- Reduce workers: 4 aggregation processors'  
    echo '- Data partitioning: Hash-based on IP address'
    echo '- Fault tolerance: Individual worker failure isolation'
} > /volumes/final-output/mapreduce_report.md

# Generate JSON results for API consumption
{
    echo '{'
    echo '  \"job_type\": \"distributed_mapreduce\",'
    echo '  \"completed_at\": \"'\$(date -Iseconds)'\",'
    echo '  \"status\": \"SUCCESS\",'
    echo '  \"workers\": {'
    echo '    \"map_workers\": 8,'
    echo '    \"reduce_workers\": 4'
    echo '  },'
    echo '  \"results\": {'
    
    # Status codes as JSON
    echo '    \"status_codes\": {'
    first=true
    while IFS=',' read -r status count; do
        if [ \"\$status\" != \"status_code\" ]; then
            [ \"\$first\" = false ] && echo ','
            echo -n \"      \\\"\$status\\\": \$count\"
            first=false
        fi
    done < /volumes/final-output/final_status_codes.csv
    echo ''
    echo '    },'
    
    if [ -f '/volumes/final-output/aggregated_metrics.csv' ]; then
        total_requests=\$(grep 'total_requests' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)
        avg_size=\$(grep 'avg_response_size' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)
        echo \"    \\\"total_requests\\\": \$total_requests,\"
        echo \"    \\\"avg_response_size\\\": \$avg_size\"
    fi
    
    echo '  }'
    echo '}'
} > /volumes/final-output/results.json

echo 'Final aggregation completed successfully'
echo 'MAPREDUCE_SUCCESS' > /volumes/mapreduce-control/final_status.txt
")

echo "✅ Final aggregation job started"
sleep 8

echo "📋 Demo 6: Fault Tolerance Demonstration"
echo "----------------------------------------"
echo "Demonstrating fault tolerance and recovery patterns"

# Simulate a failure scenario and recovery
FAULT_TOLERANCE_JOB=$(rnx run --volume=final-output --volume=mapreduce-control bash -c "
echo 'Fault Tolerance Demo: Simulating failure scenarios'

# Check if main processing completed
if [ -f '/volumes/mapreduce-control/final_status.txt' ]; then
    status=\$(cat /volumes/mapreduce-control/final_status.txt)
    echo \"Main processing status: \$status\"
    
    if [ \"\$status\" = 'MAPREDUCE_SUCCESS' ]; then
        echo 'Main processing completed successfully'
        
        # Simulate failure detection and recovery
        echo 'Simulating failure detection scenario...'
        
        # Create backup of results
        echo 'Creating backup of critical results...'
        cp /volumes/final-output/results.json /volumes/final-output/results_backup.json
        cp /volumes/final-output/mapreduce_report.md /volumes/final-output/report_backup.md
        
        # Simulate recovery verification
        echo 'Verifying result integrity...'
        
        # Check if results are valid
        if [ -s '/volumes/final-output/results.json' ] && [ -s '/volumes/final-output/final_status_codes.csv' ]; then
            echo '✅ Results integrity verified'
            
            # Add recovery metadata
            {
                echo '{'
                echo '  \"recovery\": {'
                echo '    \"verified_at\": \"'\$(date -Iseconds)'\",'
                echo '    \"backup_created\": true,'
                echo '    \"integrity_check\": \"passed\"'
                echo '  }'
                echo '}'
            } > /volumes/final-output/recovery_info.json
            
            echo 'Fault tolerance verification completed'
        else
            echo '❌ Results integrity check failed - recovery needed'
        fi
    else
        echo 'Main processing failed - implementing recovery strategy'
        
        # Implement recovery logic
        echo 'Attempting automated recovery...'
        echo 'Recovery attempt logged' > /volumes/final-output/recovery_attempt.log
    fi
else
    echo 'No processing status found - system may need reinitialization'
fi

echo 'Fault tolerance demonstration completed'
")

echo "✅ Fault tolerance demonstration started"
sleep 5

echo "📋 Demo 7: Performance Analysis"
echo "-------------------------------"
echo "Analyzing distributed processing performance"

PERFORMANCE_ANALYSIS_JOB=$(rnx run --volume=final-output --volume=mapreduce-control bash -c "
echo 'Performance Analysis: Evaluating distributed processing efficiency'

# Calculate processing metrics
start_time='\$(date +%s)'  # Current time as baseline
processing_duration=60     # Simulated processing time

# Analyze resource utilization
{
    echo '# Distributed Processing Performance Report'
    echo '=========================================='
    echo ''
    echo 'Analysis generated at: '\$(date)
    echo ''
    echo '## Resource Utilization'
    echo '- Map workers: 8 (parallel processing)'
    echo '- Reduce workers: 4 (aggregation)'
    echo '- Memory per worker: 256MB'
    echo '- CPU per worker: 25-50%'
    echo '- Total estimated processing time: ~60 seconds'
    echo ''
    echo '## Scalability Analysis'
    
    if [ -f '/volumes/final-output/aggregated_metrics.csv' ]; then
        total_requests=\$(grep 'total_requests' /volumes/final-output/aggregated_metrics.csv | cut -d',' -f2)
        throughput=\$((total_requests / 60))  # requests per second
        
        echo \"- Total records processed: \$total_requests\"
        echo \"- Estimated throughput: \$throughput records/second\"
        echo '- Processing efficiency: High (parallel execution)'
    fi
    
    echo ''
    echo '## Optimization Recommendations'
    echo '- Increase map workers for larger datasets'
    echo '- Adjust partition size based on data characteristics'
    echo '- Consider memory volume types for intermediate data'
    echo '- Implement dynamic worker scaling based on load'
    echo ''
    echo '## Fault Tolerance Features'
    echo '- Worker failure isolation'
    echo '- Partition-level recovery'
    echo '- Result validation and backup'
    echo '- Automatic retry mechanisms (can be implemented)'
    
} > /volumes/final-output/performance_analysis.md

echo 'Performance analysis completed'
")

echo "✅ Performance analysis started"
sleep 5

echo "📋 Demo 8: Results Summary"
echo "-------------------------"
echo "Displaying comprehensive distributed processing results"

echo "Final Results:"
rnx run --volume=final-output bash -c "
echo '=== Distributed Processing Summary ==='

if [ -f '/volumes/final-output/mapreduce_report.md' ]; then
    echo 'Processing Report:'
    head -20 /volumes/final-output/mapreduce_report.md
    echo ''
fi

if [ -f '/volumes/final-output/final_status_codes.csv' ]; then
    echo 'Status Code Distribution:'
    cat /volumes/final-output/final_status_codes.csv
    echo ''
fi

if [ -f '/volumes/final-output/results.json' ]; then
    echo 'JSON Results (first 10 lines):'
    head -10 /volumes/final-output/results.json
    echo ''
fi

if [ -f '/volumes/final-output/performance_analysis.md' ]; then
    echo 'Performance Analysis Summary:'
    head -15 /volumes/final-output/performance_analysis.md
fi
"

echo ""
echo "Volume cleanup (optional):"
echo "To clean up distributed processing volumes:"
echo "  rnx volume remove distributed-data"
echo "  rnx volume remove final-output"
echo "  rnx volume remove reduce-output"
echo "  # ... and other volumes as needed"
echo ""

echo "✅ Distributed Processing Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • MapReduce pattern implementation in Joblet"
echo "  • Large-scale data partitioning strategies"
echo "  • Parallel processing with multiple coordinated workers"
echo "  • Shuffle phase for data redistribution"
echo "  • Fault tolerance and recovery mechanisms"
echo "  • Performance analysis and optimization techniques"
echo "  • Scalable distributed computing patterns"
echo ""
echo "📝 Key distributed patterns:"
echo "  • Data partitioning for parallel processing"
echo "  • Map-Reduce paradigm for large datasets"
echo "  • Worker coordination and synchronization"
echo "  • Fault isolation and recovery strategies"
echo "  • Performance monitoring and optimization"
echo ""
echo "💡 Production considerations:"
echo "  • Implement dynamic worker scaling"
echo "  • Add comprehensive error handling and retries"
echo "  • Monitor resource utilization across workers"
echo "  • Use persistent storage for critical intermediate results"
echo "  • Implement data locality optimization"
echo "  • Add automated failure detection and recovery"
echo ""
echo "⚡ Performance tips:"
echo "  • Optimize partition size based on data characteristics"
echo "  • Use memory volumes for intermediate processing"
echo "  • Balance CPU and memory resources per worker"
echo "  • Consider network overhead in data shuffling"
echo "  • Monitor and tune concurrency levels"
echo ""
echo "➡️  Next: Try ./03_production_deployment.sh for production patterns"