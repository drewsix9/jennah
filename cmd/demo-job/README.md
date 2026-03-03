# Demo Job - Distributed File Processing

Phase 1 implementation of the distributed workload processing system. This application processes a large text file in parallel across multiple instances, measuring performance and validating the distributed processing pipeline.

## Overview

**Purpose:** Demonstrate instance-based parallelization working correctly across GCP Batch environment

**Business Value:**

- Validates distributed processing architecture
- Measures speedup and efficiency metrics
- Provides real-world performance benchmarks
- Tests GCP Batch integration end-to-end

## Features

### Phase 1: Single & Multi-Instance Local Processing

- ✅ Processes local text files
- ✅ Divides work across instances using byte-range distribution
- ✅ Counts lines, words, and characters per instance
- ✅ Generates per-instance metrics in JSON format
- ✅ Error handling with retry logic
- ✅ Comprehensive unit tests for chunk calculator

### Phase 2: GCS Integration

- ⏳ Read input files from Google Cloud Storage
- ⏳ Write metrics to GCS
- ⏳ Handle network timeouts and retries

### Phase 3: Aggregation & Analysis

- ⏳ Aggregate metrics across all instances
- ⏳ Calculate efficiency and speedup statistics
- ⏳ Generate performance reports

## Building

### Local Build

```bash
cd cmd/demo-job
go build -o demo-job main.go config.go
```

### Docker Build

```bash
cd ../..  # Go to workspace root
docker build -f cmd/demo-job/Dockerfile -t demo-job:latest .
```

## Running

### Prerequisites

1. Create a test input file:

```bash
python3 << 'EOF'
with open("test-input.txt", "w") as f:
    for i in range(100000):
        f.write(f"Line {i}: Lorem ipsum dolor sit amet consectetur adipiscing elit\n")
EOF
```

2. Set environment variables or use default:

```bash
export INPUT_DATA_PATH="./test-input.txt"
export BATCH_TASK_INDEX=0
export BATCH_TASK_COUNT=1
export OUTPUT_BASE_PATH="./output"
```

### Single Instance

```bash
./demo-job
```

### Multi-Instance (Local Simulation)

```bash
# Terminal 1: Instance 0 of 2
export INPUT_DATA_PATH="./test-input.txt"
export BATCH_TASK_INDEX=0
export BATCH_TASK_COUNT=2
export OUTPUT_BASE_PATH="./output"
./demo-job

# Terminal 2: Instance 1 of 2
export INPUT_DATA_PATH="./test-input.txt"
export BATCH_TASK_INDEX=1
export BATCH_TASK_COUNT=2
export OUTPUT_BASE_PATH="./output"
./demo-job
```

### Via CLI Flags

```bash
./demo-job --instance-id 0 --total-instances 4
```

## Configuration

### Environment Variables

| Variable                  | Default      | Description                |
| ------------------------- | ------------ | -------------------------- |
| `INPUT_DATA_PATH`         | (required)   | Path to input file         |
| `INPUT_DATA_SIZE`         | (from file)  | File size in bytes         |
| `BATCH_TASK_INDEX`        | 0            | Instance ID (0-based)      |
| `BATCH_TASK_COUNT`        | 1            | Total instances            |
| `OUTPUT_BASE_PATH`        | `./output`   | Output directory           |
| `JOB_ID`                  | (empty)      | Job identifier for logging |
| `DISTRIBUTION_MODE`       | `BYTE_RANGE` | Distribution strategy      |
| `ENABLE_DISTRIBUTED_MODE` | false        | Feature flag               |

### CLI Flags

```
--instance-id       Override BATCH_TASK_INDEX
--total-instances   Override BATCH_TASK_COUNT
```

## Output

### Metrics File

Each instance produces a JSON file: `output/instance-{id}.json`

```json
{
  "instance_id": 0,
  "task_index": 0,
  "task_index_max": 3,
  "job_id": "job-123",
  "start_byte": 0,
  "end_byte": 268435455,
  "bytes_processed": 268435456,
  "lines_count": 2500000,
  "words_count": 25000000,
  "characters_count": 268435456,
  "processing_time_seconds": 12.5,
  "throughput_mb_per_second": 21.5,
  "timestamp": "2026-03-02T12:34:56Z"
}
```

## Testing

### Unit Tests

```bash
cd ../../internal/demo
go test -v

# With coverage
go test -v -cover
```

### Running Individual Tests

```bash
go test -v -run TestCalculateByteRange_Normal
go test -v -run TestCalculateByteRange_Uneven
go test -v -run TestCalculateByteRange_FileSmall
go test -v -run TestCalculateByteRange_EmptyFile
go test -v -run TestCalculateByteRange_Error_InvalidInstanceID
```

### Integration Test

```bash
cd ../../test
bash demo-job-test.sh
```

## Metrics Explained

| Metric                     | Description                                |
| -------------------------- | ------------------------------------------ |
| `bytes_processed`          | Number of bytes in assigned range          |
| `lines_count`              | Total lines in assigned range              |
| `words_count`              | Total words in assigned range              |
| `characters_count`         | Total characters (runes) in assigned range |
| `processing_time_seconds`  | Time to process the range                  |
| `throughput_mb_per_second` | Processing speed (MB/s)                    |

## Troubleshooting

### Error: "INPUT_DATA_PATH not set"

- Set the environment variable: `export INPUT_DATA_PATH="./test-input.txt"`

### Error: "Input file not found"

- Verify the file path is correct and accessible
- Check file permissions: `ls -la ./test-input.txt`

### Error: "InstanceID >= TotalInstances"

- Ensure `BATCH_TASK_INDEX < BATCH_TASK_COUNT`
- Instance IDs are 0-based

### Empty metrics file

- File may be smaller than number of instances
- Empty ranges generate valid metrics with 0 counts

## Performance Notes

### Expected Performance (for 1GB file)

- **Single instance:** ~15-20 seconds
- **4 instances:** ~5-7 seconds (3x-4x speedup expected)
- **Throughput:** 50-70 MB/s per instance

### Factors Affecting Performance

- File I/O speed (SSD vs HDD)
- Network latency (for GCS in Phase 2)
- Instance type and CPU cores
- Buffer sizes and GC tuning

## Next Steps

1. **Phase 2:** Add GCS input/output support
2. **Phase 3:** Implement metrics aggregation
3. **Phase 4:** Deploy to GCP Batch with real workloads
