#!/bin/bash
set -e

echo "=== Phase 2: Multi-Instance Awareness Test ==="
echo "Testing byte-range isolation and multi-instance coordination"
echo ""

# Build demo-job
echo "Building demo-job..."
go build -o cmd/demo-job/demo-job ./cmd/demo-job 2>/dev/null
echo "✓ Build complete"
echo ""

# Generate test file (5M lines)
echo "Generating test file (5M lines)..."
python3 << 'EOF'
with open("test-phase2-5m.txt", "w") as f:
    for i in range(5000000):
        f.write(f"Line {i}: Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor\n")
print("Generated 5M line test file")
EOF

FILE_SIZE=$(stat -f%z test-phase2-5m.txt 2>/dev/null || stat -c%s test-phase2-5m.txt)
echo "File size: $FILE_SIZE bytes"
echo ""

# Test: 4 instances in parallel
echo "--- Test: 4 Concurrent Instances ---"
mkdir -p output-phase2

export INPUT_DATA_PATH="./test-phase2-5m.txt"
export INPUT_DATA_SIZE="$FILE_SIZE"
export OUTPUT_BASE_PATH="./output-phase2"

# Start all 4 instances in parallel
for i in {0..3}; do
  export BATCH_TASK_INDEX=$i
  export BATCH_TASK_COUNT=4
  echo "Starting instance $i/4..."
  ./cmd/demo-job/demo-job > /tmp/phase2_$i.log 2>&1 &
done

# Wait for all to complete
wait
echo "All instances completed"
echo ""

# Verify results
echo "--- Verification ---"
TOTAL_LINES=0
TOTAL_BYTES=0
GAPS=0

for i in {0..3}; do
  if [ -f "output-phase2/instance-$i.json" ]; then
    LINES=$(grep -o '"lines_count": [0-9]*' "output-phase2/instance-$i.json" | grep -o '[0-9]*')
    BYTES=$(grep -o '"bytes_processed": [0-9]*' "output-phase2/instance-$i.json" | grep -o '[0-9]*')
    START=$(grep -o '"start_byte": [0-9]*' "output-phase2/instance-$i.json" | grep -o '[0-9]*')
    END=$(grep -o '"end_byte": [0-9]*' "output-phase2/instance-$i.json" | grep -o '[0-9]*')
    
    echo "✓ Instance $i:"
    echo "    Byte range: $START - $END ($BYTES bytes)"
    echo "    Lines: $LINES"
    
    TOTAL_LINES=$((TOTAL_LINES + LINES))
    TOTAL_BYTES=$((TOTAL_BYTES + BYTES))
  else
    echo "✗ Instance $i output MISSING"
    exit 1
  fi
done

echo ""
echo "Summary:"
echo "  Total bytes processed: $TOTAL_BYTES / $FILE_SIZE"
echo "  Total lines counted: $TOTAL_LINES / ~5000000"

# Validate
if [ "$TOTAL_BYTES" -eq "$FILE_SIZE" ]; then
  echo "✓ All bytes accounted for (no gaps/overlaps)"
else
  echo "✗ Byte count mismatch: expected $FILE_SIZE, got $TOTAL_BYTES"
  exit 1
fi

if [ "$TOTAL_LINES" -gt 4900000 ] && [ "$TOTAL_LINES" -lt 5100000 ]; then
  echo "✓ Line count in expected range (~5M)"
else
  echo "✗ Line count out of range: $TOTAL_LINES"
  exit 1
fi

echo ""
echo "=== Phase 2 Test PASSED ✓ ==="
echo "Multi-instance byte-range processing validated"
echo ""

# Cleanup
rm -rf output-phase2 test-phase2-5m.txt /tmp/phase2_*.log
echo "Cleanup complete"
