#!/bin/bash
set -e

echo "🌍 Joblet Basic Usage: Environment Variables"
echo "============================================"
echo ""
echo "This demo shows how to use environment variables for job configuration."
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

echo "📋 Demo 1: Basic Environment Variables"
echo "--------------------------------------"
echo "Setting and using simple environment variables"

echo "Setting MESSAGE variable:"
rnx run --env=MESSAGE="Hello from Joblet!" bash -c 'echo "Received message: $MESSAGE"'
echo ""

echo "Setting multiple variables:"
rnx run --env=USER="demo" --env=ROLE="admin" --env=DEBUG="true" bash -c '
echo "Configuration:"
echo "  User: $USER"
echo "  Role: $ROLE"  
echo "  Debug mode: $DEBUG"
'
echo ""

echo "📋 Demo 2: Environment in Script Processing"
echo "-------------------------------------------"
echo "Using environment variables to control script behavior"

echo "Processing configuration via environment:"
rnx run --env=PROCESS_COUNT="5" --env=DELAY_SECONDS="1" --env=OUTPUT_FORMAT="detailed" bash -c '
echo "Starting processing with configuration:"
echo "  Process count: $PROCESS_COUNT"
echo "  Delay: $DELAY_SECONDS seconds"
echo "  Output format: $OUTPUT_FORMAT"
echo ""

for i in $(seq 1 $PROCESS_COUNT); do
    if [ "$OUTPUT_FORMAT" = "detailed" ]; then
        echo "[$i/$PROCESS_COUNT] Processing item $i at $(date +%H:%M:%S)"
    else
        echo "Processing $i"
    fi
    sleep $DELAY_SECONDS
done
echo "Processing completed"
'
echo ""

echo "📋 Demo 3: File Processing with Environment"
echo "-------------------------------------------"
echo "Using environment variables with file operations"

echo "File processing controlled by environment variables:"
rnx run --upload=sample_data.txt --env=LINES_TO_SHOW="3" --env=PATTERN="Entry" --env=OUTPUT_FILE="filtered_data.txt" bash -c '
echo "File processing configuration:"
echo "  Input file: sample_data.txt"
echo "  Lines to show: $LINES_TO_SHOW"
echo "  Filter pattern: $PATTERN"
echo "  Output file: $OUTPUT_FILE"
echo ""

echo "First $LINES_TO_SHOW lines of input:"
head -n $LINES_TO_SHOW sample_data.txt
echo ""

echo "Lines matching pattern \"$PATTERN\":"
grep "$PATTERN" sample_data.txt > $OUTPUT_FILE
echo "Found $(wc -l < $OUTPUT_FILE) matching lines"
cat $OUTPUT_FILE
echo ""

echo "Processed data saved to $OUTPUT_FILE"
ls -la $OUTPUT_FILE
'
echo ""

echo "📋 Demo 4: Application Configuration"
echo "------------------------------------"
echo "Simulating application configuration with environment variables"

echo "Application startup with environment configuration:"
rnx run --env=APP_NAME="DataProcessor" --env=VERSION="1.2.3" --env=LOG_LEVEL="INFO" --env=MAX_WORKERS="3" --env=TIMEOUT="30" bash -c '
echo "=== $APP_NAME v$VERSION ==="
echo "Starting application with configuration:"
echo ""
echo "Application Settings:"
echo "  Name: $APP_NAME"
echo "  Version: $VERSION"
echo "  Log Level: $LOG_LEVEL"
echo "  Max Workers: $MAX_WORKERS"
echo "  Timeout: $TIMEOUT seconds"
echo ""

# Simulate application startup
echo "[$LOG_LEVEL] Application startup initiated"
echo "[$LOG_LEVEL] Initializing $MAX_WORKERS worker processes"

for worker in $(seq 1 $MAX_WORKERS); do
    echo "[$LOG_LEVEL] Worker $worker initialized"
    sleep 0.5
done

echo "[$LOG_LEVEL] All workers ready"
echo "[$LOG_LEVEL] Application $APP_NAME v$VERSION started successfully"
echo "[$LOG_LEVEL] Ready to process requests (timeout: ${TIMEOUT}s)"
'
echo ""

echo "📋 Demo 5: Environment Variable Types and Formats"
echo "-------------------------------------------------"
echo "Demonstrating different types of environment variables"

echo "Various data types and formats:"
rnx run --env=STRING_VAR="Hello World" --env=NUMBER_VAR="42" --env=FLOAT_VAR="3.14159" --env=BOOLEAN_VAR="true" --env=LIST_VAR="item1,item2,item3" --env=JSON_VAR='{"key":"value","count":123}' bash -c '
echo "Environment Variable Types:"
echo ""

echo "String: $STRING_VAR"
echo "Number: $NUMBER_VAR (math: $((NUMBER_VAR * 2)))"
echo "Float: $FLOAT_VAR"
echo "Boolean: $BOOLEAN_VAR"
echo "List: $LIST_VAR"
echo "JSON: $JSON_VAR"
echo ""

# Process list variable
echo "Processing list items:"
IFS="," read -ra ITEMS <<< "$LIST_VAR"
for item in "${ITEMS[@]}"; do
    echo "  - Processing: $item"
done
echo ""

# Conditional logic based on boolean
if [ "$BOOLEAN_VAR" = "true" ]; then
    echo "Boolean condition: Feature enabled"
else
    echo "Boolean condition: Feature disabled"
fi
'
echo ""

echo "📋 Demo 6: Development vs Production Configuration"
echo "-------------------------------------------------"
echo "Simulating different environment configurations"

echo "Development environment configuration:"
rnx run --env=ENV="development" --env=DEBUG="true" --env=DB_HOST="localhost" --env=API_TIMEOUT="60" --env=LOG_LEVEL="DEBUG" bash -c '
echo "=== Environment: $ENV ==="
echo "Configuration loaded:"
echo "  Environment: $ENV"
echo "  Debug mode: $DEBUG"
echo "  Database host: $DB_HOST"
echo "  API timeout: $API_TIMEOUT seconds"
echo "  Log level: $LOG_LEVEL"
echo ""

if [ "$DEBUG" = "true" ]; then
    echo "[$LOG_LEVEL] Debug mode enabled - verbose logging active"
    echo "[$LOG_LEVEL] Development database connection: $DB_HOST"
    echo "[$LOG_LEVEL] Extended timeout for development: ${API_TIMEOUT}s"
fi
echo "[$LOG_LEVEL] Development environment ready"
'
echo ""

echo "Production environment configuration:"
rnx run --env=ENV="production" --env=DEBUG="false" --env=DB_HOST="prod-db.company.com" --env=API_TIMEOUT="10" --env=LOG_LEVEL="ERROR" bash -c '
echo "=== Environment: $ENV ==="
echo "Configuration loaded:"
echo "  Environment: $ENV"
echo "  Debug mode: $DEBUG"
echo "  Database host: $DB_HOST"
echo "  API timeout: $API_TIMEOUT seconds"
echo "  Log level: $LOG_LEVEL"
echo ""

if [ "$DEBUG" = "false" ]; then
    echo "[$LOG_LEVEL] Production mode - minimal logging"
    echo "[$LOG_LEVEL] Production database: $DB_HOST"
    echo "[$LOG_LEVEL] Optimized timeout: ${API_TIMEOUT}s"
fi
echo "[$LOG_LEVEL] Production environment ready"
'
echo ""

echo "📋 Demo 7: Environment Variable Validation"
echo "------------------------------------------"
echo "Demonstrating environment variable validation and defaults"

echo "Job with environment validation:"
rnx run --env=REQUIRED_VAR="present" --env=OPTIONAL_VAR="" bash -c '
echo "Environment Variable Validation:"
echo ""

# Check required variables
if [ -z "$REQUIRED_VAR" ]; then
    echo "ERROR: REQUIRED_VAR is not set"
    exit 1
else
    echo "✅ REQUIRED_VAR is set: $REQUIRED_VAR"
fi

# Set defaults for optional variables
OPTIONAL_VAR=${OPTIONAL_VAR:-"default_value"}
echo "✅ OPTIONAL_VAR (with default): $OPTIONAL_VAR"

# Validate numeric variables
NUMERIC_VAR=${NUMERIC_VAR:-"10"}
if ! [[ "$NUMERIC_VAR" =~ ^[0-9]+$ ]]; then
    echo "⚠️  NUMERIC_VAR is not numeric, using default: 10"
    NUMERIC_VAR=10
else
    echo "✅ NUMERIC_VAR is valid: $NUMERIC_VAR"
fi

echo ""
echo "All environment variables validated successfully"
'
echo ""

echo "📋 Demo 8: Complex Configuration Example"
echo "----------------------------------------"
echo "Real-world example with comprehensive configuration"

echo "Complex application configuration:"
rnx run --upload=sample_data.txt \
    --env=APP_NAME="DataAnalyzer" \
    --env=INPUT_FILE="sample_data.txt" \
    --env=OUTPUT_DIR="/tmp/results" \
    --env=ANALYSIS_TYPE="summary" \
    --env=ENABLE_CHARTS="false" \
    --env=MAX_LINES="100" \
    --env=WORKER_THREADS="2" \
    bash -c '
echo "=== $APP_NAME Configuration ==="
echo ""
echo "Input Configuration:"
echo "  Input file: $INPUT_FILE"
echo "  Output directory: $OUTPUT_DIR"
echo "  Analysis type: $ANALYSIS_TYPE"
echo "  Max lines to process: $MAX_LINES"
echo ""
echo "Processing Configuration:"
echo "  Worker threads: $WORKER_THREADS"
echo "  Generate charts: $ENABLE_CHARTS"
echo ""

# Create output directory
mkdir -p $OUTPUT_DIR
echo "✅ Output directory created: $OUTPUT_DIR"

# Process input file
echo ""
echo "Processing $INPUT_FILE..."
case $ANALYSIS_TYPE in
    "summary")
        echo "Performing summary analysis:"
        echo "  Total lines: $(wc -l < $INPUT_FILE)"
        echo "  Total words: $(wc -w < $INPUT_FILE)"
        echo "  Total characters: $(wc -c < $INPUT_FILE)"
        ;;
    "detailed")
        echo "Performing detailed analysis:"
        head -n $MAX_LINES $INPUT_FILE > $OUTPUT_DIR/processed_data.txt
        echo "  Processed first $MAX_LINES lines"
        echo "  Output saved to: $OUTPUT_DIR/processed_data.txt"
        ;;
    *)
        echo "Unknown analysis type: $ANALYSIS_TYPE"
        ;;
esac

# Simulate worker threads
echo ""
echo "Using $WORKER_THREADS worker threads for processing:"
for worker in $(seq 1 $WORKER_THREADS); do
    echo "  Worker $worker: Processing completed"
done

echo ""
echo "=== Analysis Complete ==="
ls -la $OUTPUT_DIR/ 2>/dev/null || echo "Output directory contents listed above"
'
echo ""

echo "✅ Environment Variables Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to set environment variables with --env"
echo "  • Using multiple environment variables in a single job"
echo "  • Environment-driven script behavior and configuration"
echo "  • Different data types in environment variables"
echo "  • Validation and default values for environment variables"
echo "  • Development vs production configuration patterns"
echo "  • Complex application configuration examples"
echo ""
echo "📝 Key takeaways:"
echo "  • Environment variables provide flexible job configuration"
echo "  • Use --env=KEY=VALUE syntax for each variable"
echo "  • Variables are available as \$KEY or \${KEY} in job scripts"
echo "  • Environment variables work with file uploads and volumes"
echo "  • Always validate critical environment variables"
echo "  • Use meaningful variable names and document their purpose"
echo ""
echo "💡 Best practices:"
echo "  • Validate required environment variables at job start"
echo "  • Provide sensible defaults for optional variables"
echo "  • Use consistent naming conventions (e.g., UPPER_CASE)"
echo "  • Document expected environment variables"
echo "  • Never pass sensitive data like passwords through environment variables"
echo "  • Use environment variables for configuration, not secrets"
echo ""
echo "🔧 Common patterns:"
echo "  --env=ENV=production                    # Environment type"
echo "  --env=DEBUG=true                        # Feature flags"
echo "  --env=MAX_WORKERS=4                     # Numeric configuration"
echo "  --env=INPUT_FILE=data.csv               # File paths"
echo "  --env=OUTPUT_FORMAT=json                # Format options"
echo "  --env=CONFIG='{\"key\":\"value\"}'         # JSON configuration"
echo ""
echo "⚠️  Security reminder:"
echo "  • Never pass passwords, API keys, or secrets via --env"
echo "  • Use secure secret management systems for sensitive data"
echo "  • Environment variables may be visible in process lists"
echo ""
echo "➡️  Next: Try ./07_network_basics.sh to learn about network isolation"