// Package router provides job complexity classification and GCP service routing.
//
// EvaluateJobComplexity inspects an incoming SubmitJobRequest and returns the
// appropriate ComplexityLevel together with the GCP service that should execute
// the workload:
//
//   - SIMPLE  → Cloud Run Jobs (no machine type, or resources within Cloud Run limits)
//   - COMPLEX → Cloud Batch   (specific machine type, heavy resources, or long duration)
package router

import (
	"context"
	"fmt"

	jennahv1 "github.com/alphauslabs/jennah/gen/proto"
)

// ComplexityLevel represents the tier of a submitted job.
type ComplexityLevel int

const (
	ComplexityUnspecified ComplexityLevel = iota
	// ComplexitySimple: no machine-type constraint; all resources within Cloud Run Jobs limits (≤4000 mCPU, ≤8192 MiB, ≤1 hr).
	ComplexitySimple
	// ComplexityComplex: explicit machine type, heavy CPU/memory, or very long duration.
	ComplexityComplex
)

// String returns a human-readable label for the complexity level.
func (c ComplexityLevel) String() string {
	switch c {
	case ComplexitySimple:
		return "SIMPLE"
	case ComplexityComplex:
		return "COMPLEX"
	default:
		return "UNSPECIFIED"
	}
}

// AssignedService represents the GCP execution service that will run the job.
type AssignedService int

const (
	AssignedServiceUnspecified AssignedService = iota
	_ // reserved (formerly Cloud Tasks)
	// AssignedServiceCloudRunJob routes the job to GCP Cloud Run Jobs.
	AssignedServiceCloudRunJob
	// AssignedServiceCloudBatch routes the job to GCP Cloud Batch.
	AssignedServiceCloudBatch
)

// String returns a human-readable label for the assigned service.
func (a AssignedService) String() string {
	switch a {
	case AssignedServiceCloudRunJob:
		return "CLOUD_RUN_JOB"
	case AssignedServiceCloudBatch:
		return "CLOUD_BATCH"
	default:
		return "UNSPECIFIED"
	}
}

// RoutingDecision is the output of EvaluateJobComplexity.
type RoutingDecision struct {
	Complexity      ComplexityLevel
	AssignedService AssignedService
	// Reason is a short human-readable explanation of why this tier was chosen.
	Reason string
}

// Thresholds that define tier boundaries.
//
// These constants are exported so that callers (e.g. tests, dashboards) can
// reference and override them without magic numbers.
const (
	// MediumCPUMillisMax is the maximum CPU (in milli-cores) for a SIMPLE job (above this → COMPLEX).
	MediumCPUMillisMax int64 = 4000
	// MediumMemoryMiBMax is the maximum memory (in MiB) for a SIMPLE job (above this → COMPLEX).
	MediumMemoryMiBMax int64 = 8192
	// MediumDurationSecMax is the maximum duration (in seconds) for a SIMPLE job (above this → COMPLEX).
	MediumDurationSecMax int64 = 3600
)

// EvaluateJobComplexity inspects req and returns the routing decision.
//
// Decision logic (strictest check first):
//  1. If machine_type is set → COMPLEX / Cloud Batch.
//  2. If cpu_millis > MediumCPUMillisMax, memory_mib > MediumMemoryMiBMax,
//     or max_run_duration_seconds > MediumDurationSecMax → COMPLEX / Cloud Batch.
//  3. Otherwise → SIMPLE / Cloud Run Jobs.
//
// Zero-value resource fields are treated as "not specified" and do not push
// the job into a higher tier on their own.
func EvaluateJobComplexity(req *jennahv1.SubmitJobRequest) RoutingDecision {
	// Extract resource values; fall back to zero when no override is provided.
	var cpuMillis, memoryMiB, durationSec int64
	if ro := req.GetResourceOverride(); ro != nil {
		cpuMillis = ro.GetCpuMillis()
		memoryMiB = ro.GetMemoryMib()
		durationSec = ro.GetMaxRunDurationSeconds()
	}
	machineType := req.GetMachineType()

	// --- Rule 1: explicit machine type → always COMPLEX ---
	if machineType != "" {
		return RoutingDecision{
			Complexity:      ComplexityComplex,
			AssignedService: AssignedServiceCloudBatch,
			Reason:          "explicit machine_type requested: " + machineType,
		}
	}

	// --- Rule 2: heavy resources → COMPLEX ---
	if exceedsThreshold(cpuMillis, MediumCPUMillisMax) {
		return RoutingDecision{
			Complexity:      ComplexityComplex,
			AssignedService: AssignedServiceCloudBatch,
			Reason:          "cpu_millis exceeds medium threshold",
		}
	}
	if exceedsThreshold(memoryMiB, MediumMemoryMiBMax) {
		return RoutingDecision{
			Complexity:      ComplexityComplex,
			AssignedService: AssignedServiceCloudBatch,
			Reason:          "memory_mib exceeds medium threshold",
		}
	}
	if exceedsThreshold(durationSec, MediumDurationSecMax) {
		return RoutingDecision{
			Complexity:      ComplexityComplex,
			AssignedService: AssignedServiceCloudBatch,
			Reason:          "max_run_duration_seconds exceeds medium threshold",
		}
	}

	// --- Rule 3: everything else → SIMPLE (Cloud Run Jobs) ---
	return RoutingDecision{
		Complexity:      ComplexitySimple,
		AssignedService: AssignedServiceCloudRunJob,
		Reason:          "no machine type, resources within Cloud Run Jobs limits",
	}
}

// EvaluateJobComplexityWithGemini classifies the job using the Gemini AI API
// and maps the result to a RoutingDecision. It falls back to the deterministic
// EvaluateJobComplexity if the API call fails.
//
// apiKey is the GEMINI_API_KEY environment variable value.
func EvaluateJobComplexityWithGemini(ctx context.Context, apiKey string, req *jennahv1.SubmitJobRequest) RoutingDecision {
	var cpuMillis, memoryMiB, durationSec int64
	if ro := req.GetResourceOverride(); ro != nil {
		cpuMillis = ro.GetCpuMillis()
		memoryMiB = ro.GetMemoryMib()
		durationSec = ro.GetMaxRunDurationSeconds()
	}

	classification, err := ClassifyWithGemini(ctx, apiKey, cpuMillis, memoryMiB, durationSec, req.GetMachineType())
	if err != nil {
		// Fall back to deterministic classifier on any Gemini error.
		fallback := EvaluateJobComplexity(req)
		fallback.Reason = fmt.Sprintf("Gemini unavailable (%v); fallback: %s", err, fallback.Reason)
		return fallback
	}

	switch classification.Complexity {
	case "SIMPLE":
		return RoutingDecision{
			Complexity:      ComplexitySimple,
			AssignedService: AssignedServiceCloudRunJob,
			Reason:          classification.Reason,
		}
	case "MEDIUM":
		// MEDIUM is collapsed into SIMPLE — both route to Cloud Run Jobs.
		return RoutingDecision{
			Complexity:      ComplexitySimple,
			AssignedService: AssignedServiceCloudRunJob,
			Reason:          classification.Reason,
		}
	case "COMPLEX":
		return RoutingDecision{
			Complexity:      ComplexityComplex,
			AssignedService: AssignedServiceCloudBatch,
			Reason:          classification.Reason,
		}
	default:
		fallback := EvaluateJobComplexity(req)
		fallback.Reason = fmt.Sprintf("Gemini returned unknown tier %q; fallback: %s", classification.Complexity, fallback.Reason)
		return fallback
	}
}

// exceedsThreshold returns true only when value is both non-zero and greater
// than max. A zero value means "not specified" and is not penalised.
func exceedsThreshold(value, max int64) bool {
	return value > 0 && value > max
}
