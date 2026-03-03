#!/bin/bash
set -e

echo "=== Phase 1 Integration Test ==="
echo "Testing distributed file processing with demo-job"
echo ""

# Build demo-job if not already built
if [ ! -f "cmd/demo-job/demo-job" ]; then
    echo "Building demo-job..."
    cd cmd/demo-job
    go build -o demo-job main.go
    cd ../..
fi

# Generate test file (100,000 lines)
echo "Generating test file (100,000 lines)..."
python3 << 'EOF'
with open("test-integration-input.txt", "w") as f:
    for i in range(100000):
        f.write(f"Line {i}: The quick brown fox jumps over the lazy dog. This is test data for distributed processing.\n")
print("Generated: test-integration-input.txt (100,000 lines)")
EOF

FILE_SIZE=$(stat -f%z test-integration-input.txt 2>/dev/null || stat -c%s test-integration-input.txt 2>/dev/null)
echo "File size: $FILE_SIZE bytes"
echo ""

# Test 1: Single Instance
echo "--- Test 1: Single Instance Mode ---"
export INPUT_DATA_PATH="./test-integration-input.txt"
export INPUT_DATA_SIZE="$FILE_SIZE"
export BATCH_TASK_INDEX=0
export BATCH_TASK_COUNT=1
export OUTPUT_BASE_PATH="./output-test-single"
export JOB_ID="integration-test-single"

mkdir -p "$OUTPUT_BASE_PATH"
echo "Running single instance (0/1)..."
./cmd/demo-job/demo-job > /tmp/test1.log 2>&1 || true

if [ -f "$OUTPUT_BASE_PATH/instance-0.json" ]; then
    echo "✓ Metrics file created: $OUTPUT_BASE_PATH/instance-0.json"
    LINES=$(grep -o '"lines_count": [0-9]*' "$OUTPUT_BASE_PATH/instance-0.json" | grep -o '[0-9]*')
    WORDS=$(grep -o '"words_count": [0-9]*' "$OUTPUT_BASE_PATH/instance-0.json" | grep -o '[0-9]*')
    THROUGHPUT=$(grep -o '"throughput_mb_per_second": [0-9.]*' "$OUTPUT_BASE_PATH/instance-0.json" | grep -o '[0-9.]*')
    echo "  Lines counted: $LINES"
    echo "  Words counted: $WORDS"
    echo "  Throughput: $THROUGHPUT MB/s"
    TEST1_PASS=1
else
    echo "✗ Metrics file NOT created"
    TEST1_PASS=0
fi
echo ""

# Test 2: Multi-Instance (2 instances)
echo "--- Test 2: Multi-Instance Mode (2 instances) ---"
export OUTPUT_BASE_PATH="./output-test-multi"
mkdir -p "$OUTPUT_BASE_PATH"

echo "Running instance 0/2..."
export BATCH_TASK_INDEX=0
export BATCH_TASK_COUNT=2
./cmd/demo-job/demo-job > /tmp/test2a.log 2>&1 || true

echo "Running instance 1/2..."
export BATCH_TASK_INDEX=1
export BATCH_TASK_COUNT=2
./cmd/demo-job/demo-job > /tmp/test2b.log 2>&1 || true

if [ -f "$OUTPUT_BASE_PATH/instance-0.json" ] && [ -f "$OUTPUT_BASE_PATH/instance-1.json" ]; then
    echo "✓ Both metrics files created"
    
    LINES0=$(grep -o '"lines_count": [0-9]*' "$OUTPUT_BASE_PATH/instance-0.json" | head -1 | grep -o '[0-9]*')
    LINES1=$(grep -o '"lines_count": [0-9]*' "$OUTPUT_BASE_PATH/instance-1.json" | head -1 | grep -o '[0-9]*')
    TOTAL_LINES=$((LINES0 + LINES1))
    
    echo "  Instance 0: $LINES0 lines"
    echo "  Instance 1: $LINES1 lines"
    echo "  Total: $TOTAL_LINES lines"
    
    if [ "$TOTAL_LINES" -eq 100000 ]; then
        echo "✓ Line counts match expected (100,000)"
        TEST2_PASS=1
    else
        echo "✗ Line count mismatch: expected 100000, got $TOTAL_LINES"
        TEST2_PASS=0
    fi
else
    echo "✗ Not all metrics files created"
    TEST2_PASS=0
fi
echo ""

# Test 3: Multi-Instance (4 instances)
echo "--- Test 3: Multi-Instance Mode (4 instances) ---"
export OUTPUT_BASE_PATH="./output-test-4way"
mkdir -p "$OUTPUT_BASE_PATH"

for i in 0 1 2 3; do
    export BATCH_TASK_INDEX=$i
    export BATCH_TASK_COUNT=4
    echo "Running instance $i/4..."
    ./cmd/demo-job/demo-job > /tmp/test3_$i.log 2>&1 || true
done

ALL_FILES_EXIST=1
TOTAL_LINES=0
for i in 0 1 2 3; do
    if [ ! -f "$OUTPUT_BASE_PATH/instance-$i.json" ]; then
        ALL_FILES_EXIST=0
        break
    fi
    LINES=$(grep -o '"lines_count": [0-9]*' "$OUTPUT_BASE_PATH/instance-$i.json" | head -1 | grep -o '[0-9]*')
    TOTAL_LINES=$((TOTAL_LINES + LINES))
    echo "  Instance $i: $LINES lines"
done

if [ "$ALL_FILES_EXIST" -eq 1 ] && [ "$TOTAL_LINES" -eq 100000 ]; then
    echo "✓ All 4 instances ran successfully"
    echo "✓ Total lines: $TOTAL_LINES (expected 100,000)"
    TEST3_PASS=1
else
    echo "✗ Multi-instance test failed"
    TEST3_PASS=0
fi
echo ""

# Summary
echo "=== Test Results ==="
echo "Test 1 (Single Instance):     $([ "$TEST1_PASS" -eq 1 ] && echo '✓ PASS' || echo '✗ FAIL')"
echo "Test 2 (2 Instances):         $([ "$TEST2_PASS" -eq 1 ] && echo '✓ PASS' || echo '✗ FAIL')"
echo "Test 3 (4 Instances):         $([ "$TEST3_PASS" -eq 1 ] && echo '✓ PASS' || echo '✗ FAIL')"
echo ""

# Cleanup
echo "Cleaning up test files..."
rm -f test-integration-input.txt
rm -rf output-test-*
rm -f /tmp/test*.log

if [ "$TEST1_PASS" -eq 1 ] && [ "$TEST2_PASS" -eq 1 ] && [ "$TEST3_PASS" -eq 1 ]; then
    echo "=== Phase 1 Integration Test PASSED ✓ ==="
    exit 0
else
    echo "=== Phase 1 Integration Test FAILED ✗ ==="
    exit 1
fi
