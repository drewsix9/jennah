# Phase 3: GCS Integration Implementation

**Status:** Ready to Start  
**Estimated Duration:** 1-2 days  
**Date Started:** March 2, 2026

## Overview

Phase 3 extends the demo-job to support reading input files from Google Cloud Storage (GCS) and writing metrics output to GCS. This enables the application to work in cloud environments without depending on local filesystems.

## What's Implemented

### New Module: `internal/demo/gcs.go`

Provides GCS utilities:

- **ParseGCSPath()** - Parse `gs://bucket/key` format paths
- **IsGCSPath()** - Check if path is GCS (vs local file)
- **NewGCSRangeReader()** - Open GCS file with byte-range support
- **GetGCSObjectSize()** - Fetch GCS object metadata
- **WriteGCSFile()** - Write data to GCS
- **ListGCSObjects()** - List files in GCS directory

### Updated: `internal/demo/processor.go`

Enhanced to support GCS:

- **openFile()** - Detects local vs GCS paths and opens appropriately
- **openGCSFile()** - Opens GCS file with byte-range reader
- **WriteMetrics()** - Routes to local or GCS based on path
- **writeMetricsToGCS()** - Uploads metrics to GCS with retry logic

## Testing Phase 3

### Unit Tests

```bash
# Run GCS path parsing tests
go test -run TestGCS ./internal/demo -v

# All tests should pass
# - TestParseGCSPath_Valid (3 test cases)
# - TestParseGCSPath_Invalid (4 test cases)
# - TestIsGCSPath (6 test cases)
```

### Integration Test

The Phase 3 test supports both local validation and real GCS testing:

```powershell
# Local validation (no GCS credentials needed)
powershell -ExecutionPolicy Bypass -File test/test-phase3-gcs.ps1

# Real GCS test (requires GCS bucket and credentials)
$env:GCS_BUCKET = "gs://your-project-bucket"
powershell -ExecutionPolicy Bypass -File test/test-phase3-gcs.ps1
```

**Expected output for local test:**

```
=== Phase 3: GCS Integration Test ===
Building demo-job with GCS support...
✓ Build complete

--- Test 1: GCS Path Parsing ---
✓ TestParseGCSPath_Valid
✓ TestParseGCSPath_Invalid
✓ TestIsGCSPath

--- Test 2: Skipped (set GCS_BUCKET to test with real GCS) ---
  To enable GCS test:
    Set-Item -Path Env:GCS_BUCKET -Value "gs://your-bucket"

✓ Phase 3 GCS integration ready
  - GCS path parsing tested
  - Local file operations verified
  - GCS API integrated (ready for real bucket)
```

## Using GCS Paths

### Reading from GCS

```bash
export INPUT_DATA_PATH="gs://my-bucket/data/input.txt"
export INPUT_DATA_SIZE=1073741824  # Optional (auto-fetched if not set)
export BATCH_TASK_INDEX=0
export BATCH_TASK_COUNT=4
./cmd/demo-job/demo-job
```

**Features:**

- Auto-detects GCS paths (no special flags needed)
- Auto-fetches file size if `INPUT_DATA_SIZE` not set
- Uses byte-range readers for efficient network I/O
- Automatic retry on network failures

### Writing to GCS

```bash
export OUTPUT_BASE_PATH="gs://my-bucket/results/job-123"
./cmd/demo-job/demo-job
```

**Output files created:**

```
gs://my-bucket/results/job-123/instance-0.json
gs://my-bucket/results/job-123/instance-1.json
gs://my-bucket/results/job-123/instance-2.json
gs://my-bucket/results/job-123/instance-3.json
```

## GCS Setup Requirements

### Prerequisites

1. **GCS Bucket Created:**

   ```bash
   gsutil mb gs://your-project-bucket
   ```

2. **IAM Permissions:**
   - `roles/storage.objectViewer` - Read input files
   - `roles/storage.objectCreator` - Write output files
   - Assign to service account running demo-job

3. **Authentication:**
   - Set `GOOGLE_APPLICATION_CREDENTIALS` to service account JSON
   - Or use Application Default Credentials (if on GCP)
   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
   ```

### Example: Setup Test Bucket

```bash
# Create project
PROJECT_ID="my-demo-project"
BUCKET_NAME="${PROJECT_ID}-demo-job"

# Create bucket
gsutil mb gs://${BUCKET_NAME}

# Create service account
gcloud iam service-accounts create demo-job-sa \
  --description="Demo job GCS access" \
  --project=${PROJECT_ID}

# Grant permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:demo-job-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:demo-job-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/storage.objectCreator"

# Create and download key
gcloud iam service-accounts keys create demo-job-key.json \
  --iam-account=demo-job-sa@${PROJECT_ID}.iam.gserviceaccount.com

# Export for use
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/demo-job-key.json"
```

## Example: Multi-Instance GCS Processing

```bash
# Upload test data
gsutil cp test-data.txt gs://my-bucket/input/data.txt

# Run 4 instances in parallel
for i in {0..3}; do
  export BATCH_TASK_INDEX=$i
  export BATCH_TASK_COUNT=4
  export INPUT_DATA_PATH="gs://my-bucket/input/data.txt"
  export OUTPUT_BASE_PATH="gs://my-bucket/output/job-001"
  ./cmd/demo-job/demo-job &
done
wait

# Check output
gsutil ls gs://my-bucket/output/job-001/
gsutil cat gs://my-bucket/output/job-001/instance-0.json | jq .
```

## Error Handling

### Network Errors

GCS operations automatically retry on transient failures:

- Timeout errors: retry with exponential backoff (1s, 2s, 4s)
- Connection errors: automatic retry
- Authentication errors: fail immediately with clear message

### File Not Found

```
ERROR: Input file not found - check INPUT_DATA_PATH: storage: object not found
```

**Solution:** Verify bucket and key exist:

```bash
gsutil ls gs://bucket/key
```

### Permission Errors

```
ERROR: Permission denied - check GCS credentials and IAM roles: permission denied
```

**Solution:** Check IAM permissions and service account credentials

## Performance Considerations

### Byte-Range Reads

GCS byte-range reads are efficient:

- Only download needed bytes for this instance
- Example: 4 instances × 1GB file = each reads ~256MB
- Reduces bandwidth usage by ~4x vs full file transfer

### Throughput

Expected performance on GCS:

- **Local SSD:** 50-100 MB/s per instance
- **GCS network:** 20-50 MB/s per instance (depends on region)
- **4 concurrent instances:** 100-200 MB/s total

## Next: Phase 4

Phase 4 adds **metrics aggregation**:

- Tool to read instance-\*.json files from GCS
- Calculate speedup and efficiency
- Generate performance reports

## Git Workflow for Phase 3

```bash
# Create Phase 3 branch
git checkout -b feature/r9ight-phase3-gcs

# Run unit and integration tests
go test -run TestGCS ./internal/demo -v
powershell -ExecutionPolicy Bypass -File test/test-phase3-gcs.ps1

# Commit changes
git add internal/demo/gcs.go internal/demo/gcs_test.go cmd/demo-job/
git add test/test-phase3-gcs.ps1 docs/r9ight/PHASE_3_GCS_INTEGRATION.md

git commit -m "feat: Phase 3 - GCS integration for input and output

- Implement GCS path parsing and utilities
- Add GCS range reader for byte-range file access
- Update processor to support local/GCS input files
- Implement GCS metrics upload with retry logic
- Add comprehensive GCS unit tests
- Create GCS integration test script
- All Phase 3 tests passing"

git push origin feature/r9ight-phase3-gcs
```

## Checklist

- [ ] GCS module compiles and builds
- [ ] All GCS unit tests pass (13 test cases)
- [ ] Local file operations still work (backward compatible)
- [ ] GCS path parsing tests pass
- [ ] Integration test builds and runs
- [ ] Demo-job accepts `gs://` paths
- [ ] Metrics uploaded to GCS successfully

## References

- GCS documentation: https://cloud.google.com/storage/docs
- Google Cloud Go client: https://pkg.go.dev/cloud.google.com/go/storage
- Byte-range reads: https://cloud.google.com/storage/docs/object-lock
