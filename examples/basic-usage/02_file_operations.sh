#!/bin/bash
set -e

echo "📁 Joblet Basic Usage: File Operations"
echo "======================================"
echo ""
echo "This demo shows how to upload files and work with the job workspace."
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

if [ ! -f "sample_data.txt" ]; then
    echo "❌ Error: sample_data.txt not found"
    echo "Please ensure you're running this script from the basic-usage directory"
    exit 1
fi

echo "✅ Prerequisites checked"
echo ""

echo "📋 Demo 1: Single File Upload"
echo "-----------------------------"
echo "Uploading and displaying sample_data.txt"
rnx run --upload=sample_data.txt cat sample_data.txt
echo ""

echo "📋 Demo 2: File Analysis"
echo "------------------------"
echo "Counting lines, words, and characters in uploaded file"
rnx run --upload=sample_data.txt wc sample_data.txt
echo ""

echo "📋 Demo 3: Workspace Exploration"
echo "--------------------------------"
echo "Exploring the job workspace structure"
rnx run --upload=sample_data.txt bash -c "
echo 'Current directory:' && pwd
echo ''
echo 'Directory contents:' && ls -la
echo ''
echo 'File details:' && file sample_data.txt
"
echo ""

echo "📋 Demo 4: File Processing"
echo "--------------------------"
echo "Processing file with various commands"
rnx run --upload=sample_data.txt bash -c "
echo '=== First 5 lines ==='
head -5 sample_data.txt
echo ''
echo '=== Last 5 lines ==='
tail -5 sample_data.txt
echo ''
echo '=== Lines containing \"Entry\" ==='
grep 'Entry' sample_data.txt || echo 'No matches found'
"
echo ""

echo "📋 Demo 5: Creating Files in Workspace"
echo "--------------------------------------"
echo "Creating and processing new files within the job"
rnx run --upload=sample_data.txt bash -c "
echo 'Creating a new file...'
echo 'This is a generated file' > generated.txt
echo 'Processing complete' >> generated.txt
echo ''
echo 'New file contents:'
cat generated.txt
echo ''
echo 'Workspace now contains:'
ls -la
"
echo ""

echo "📋 Demo 6: Multiple File Processing"
echo "-----------------------------------"
echo "Working with multiple files in a single job"

# Create a temporary second file for demo
cat > temp_file2.txt << 'EOF'
Second sample file
Line 2 of second file
Line 3 of second file
EOF

rnx run --upload=sample_data.txt --upload=temp_file2.txt bash -c "
echo 'Files in workspace:'
ls -la *.txt
echo ''
echo 'Comparing file sizes:'
wc -l *.txt
echo ''
echo 'Combining files:'
cat sample_data.txt temp_file2.txt > combined.txt
echo 'Combined file has:' && wc -l combined.txt
"

# Clean up temporary file
rm -f temp_file2.txt
echo ""

echo "📋 Demo 7: Directory Upload"
echo "---------------------------"
echo "Creating a sample directory and uploading it"

# Create a sample directory structure
mkdir -p demo_dir/subdir
echo "File 1 content" > demo_dir/file1.txt
echo "File 2 content" > demo_dir/file2.txt
echo "Subdirectory file" > demo_dir/subdir/file3.txt

rnx run --upload-dir=demo_dir bash -c "
echo 'Uploaded directory structure:'
find demo_dir -type f -exec echo '{}:' \; -exec cat {} \; -exec echo '' \;
echo ''
echo 'Directory tree:'
ls -laR demo_dir/
"

# Clean up sample directory
rm -rf demo_dir
echo ""

echo "📋 Demo 8: File Processing Pipeline"
echo "-----------------------------------"
echo "Demonstrating a multi-step file processing workflow"
rnx run --upload=sample_data.txt bash -c "
echo 'Step 1: Analyze input file'
wc sample_data.txt
echo ''

echo 'Step 2: Extract specific information'
grep -i 'joblet' sample_data.txt > joblet_lines.txt || echo 'No Joblet references found'
echo 'Joblet references:' && cat joblet_lines.txt 2>/dev/null || echo 'None'
echo ''

echo 'Step 3: Create summary'
{
    echo 'File Processing Summary'
    echo '======================'
    echo 'Original file: sample_data.txt'
    echo 'Lines:' \$(wc -l < sample_data.txt)
    echo 'Words:' \$(wc -w < sample_data.txt)
    echo 'Processing date:' \$(date)
} > summary.txt

echo 'Processing summary:'
cat summary.txt
"
echo ""

echo "✅ File Operations Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to upload single files with --upload"
echo "  • How to upload directories with --upload-dir"
echo "  • Understanding the /work workspace directory"
echo "  • File processing within isolated job environments"
echo "  • Creating and manipulating files during job execution"
echo ""
echo "📝 Key takeaways:"
echo "  • Uploaded files appear in the job's working directory (/work)"
echo "  • Files created during job execution are temporary (unless saved to volumes)"
echo "  • Standard Unix file commands work as expected"
echo "  • Multiple files can be uploaded and processed together"
echo "  • Complex file processing pipelines can be built"
echo ""
echo "➡️  Next: Try ./03_resource_limits.sh to learn about resource management"