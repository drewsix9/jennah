# Aggregator Tool - Phase 4

The aggregator reads all instance metrics JSON files and calculates aggregated performance statistics.

## Purpose

After distributed job execution, each instance produces an `instance-{id}.json` file with its processing metrics. The aggregator consolidates these results and calculates:

- **Total metrics**: Lines, words, characters, bytes processed
- **Timing statistics**: Min/max/avg processing time
- **Performance metrics**: Speedup vs baseline, efficiency per instance

## Building

```bash
cd jennah
go build -o cmd/aggregator/aggregator ./cmd/aggregator
```

## Usage

### Basic usage (local files)

```bash
./cmd/aggregator/aggregator --metrics-path ./output/job-123
```

### GCS bucket usage

```bash
./cmd/aggregator/aggregator --metrics-path gs://mybucket/jobs/job-123/output
```

### With speedup calculation

```bash
./cmd/aggregator/aggregator \
  --metrics-path gs://mybucket/jobs/job-123/output \
  --baseline-seconds 45.3
```

### Summary output

```bash
./cmd/aggregator/aggregator \
  --metrics-path ./output/job-123 \
  --format summary
```

## Command-line Flags

| Flag                 | Type    | Default    | Description                                                  |
| -------------------- | ------- | ---------- | ------------------------------------------------------------ |
| `--metrics-path`     | string  | (required) | Path to metrics directory (local path or `gs://bucket/path`) |
| `--baseline-seconds` | float64 | 0          | Single-instance baseline time for speedup calculation        |
| `--format`           | string  | `detailed` | Output format: `detailed` or `summary`                       |

## Input Files

Expects files in the directory with pattern `instance-{0,1,2,...}.json`:

```json
{
  "instance_id": 0,
  "task_index": 0,
  "task_index_max": 3,
  "start_byte": 0,
  "end_byte": 268435455,
  "bytes_processed": 268435456,
  "lines_count": 1250000,
  "words_count": 5000000,
  "characters_count": 268435456,
  "processing_time_seconds": 23.456,
  "throughput_mb_per_second": 11.45,
  "timestamp": "2024-01-15T10:30:45Z"
}
```

## Output Formats

### Detailed Format (default)

```
=== AGGREGATED METRICS ===

Instances processed: 4

--- Totals ---
Total lines:      5000000
Total words:      20000000
Total characters: 268435456
Total bytes:      268435456

--- Timing ---
Min processing time:  21.234 seconds
Max processing time:  25.123 seconds
Avg processing time:  23.450 seconds

--- Performance ---
Speedup:   1.93x
Efficiency: 48.2%

--- Instance Breakdown ---

Instance 0:
  Lines:            1250000
  Words:            5000000
  Characters:       268435456
  Bytes processed:  268435456 (256.000 MB)
  Processing time:  23.456 seconds
  Throughput:       10.92 MB/s

... (Instance 1, 2, 3)

--- JSON Output ---
{
  "instances": [
    { "instance_id": 0, ... },
    { "instance_id": 1, ... },
    ...
  ],
  "total_lines": 5000000,
  "total_words": 20000000,
  ...
}
```

### Summary Format

```
Instances: 4
Total lines: 5000000
Total bytes: 268435456 (256.000 MB)
Processing time: 25.123 seconds (avg: 23.450)
Speedup: 1.93x | Efficiency: 48.2%
```

## Examples

### Example 1: Aggregate local test results

```bash
# Run Phase 3 test first (creates instance-*.json files)
bash test/test-phase3-gcs.ps1

# Then aggregate
./cmd/aggregator/aggregator --metrics-path ./output

# Output:
# Instances: 4
# Total lines: 5000000
# ...
```

### Example 2: Aggregate GCS results with speedup calculation

```bash
# After running 4-instance job on GCS:
./cmd/aggregator/aggregator \
  --metrics-path gs://myproject-jennah/jobs/distributed-001/metrics \
  --baseline-seconds 45.3 \
  --format summary

# Output:
# Instances: 4
# Total lines: 5000000
# Total bytes: 1073741824 (1024.000 MB)
# Processing time: 23.450 seconds (avg: 22.800)
# Speedup: 1.98x | Efficiency: 49.5%
```

### Example 3: Compare different instance counts

```bash
# Run with 2 instances
# ... 35 seconds

# Run with 4 instances
# ... 18 seconds

# Run with 8 instances
# ... 10 seconds

# Speedup progression:
# 2 instances: 35/35 = 1.0x, eff = 50%
# 4 instances: 35/18 = 1.94x, eff = 48.5%
# 8 instances: 35/10 = 3.5x, eff = 43.75%
```

## Interpreting Results

### Speedup

Expected speedup with N instances: **1.8x - 2.0x** for N=2, **3.5x - 3.8x** for N=4

If speedup is lower than expected:

- Check for network latency (GCS range reads)
- Look for CPU contention (all instances on same machine)
- Verify load distribution (bytes distributed evenly)

### Efficiency

Efficiency = Speedup ÷ Number of Instances

- **90-100%**: Excellent (near-linear scaling)
- **75-90%**: Good (acceptable overhead)
- **50-75%**: Fair (significant overhead)
- **<50%**: Poor (reconsider strategy)

### Throughput

MB/s = Bytes Processed ÷ Processing Time

Compare per-instance throughput:

- Should be consistent across instances
- If one instance is slower, check for:
  - Larger byte range assigned
  - Network issues to that instance
  - System resource contention

## Exit Codes

- `0`: Success - metrics aggregated
- `1`: Error - missing or invalid metrics files

## See Also

- [Phase 3: GCS Integration](../PHASE_3_GCS_INTEGRATION.md)
- [Phase 2: Multi-Instance Design](../distributed-job-design.md)
- [Metrics Schema](../../docs/backend-field-reference.md)
