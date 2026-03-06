package router

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

const geminiSystemInstruction = `You are a highly analytical and deterministic Cloud Infrastructure Job Router. You evaluate incoming computing workloads and make strict routing decisions without deviation.

Your system is receiving job payloads containing CPU (in mCPU), Memory (in MiB), Duration (in minutes), and Machine Type requests. You must classify the job's complexity based strictly on the following boundary rules:

The SIMPLE Tier (Strict Logical AND):
To be classified as SIMPLE, a job must stay under ALL of these minimum thresholds simultaneously. If it breaks even one rule, it is rejected from the Simple tier:
- CPU: 500 mCPU or less.
- Memory: 512 MiB or less.
- Duration: 10 minutes or less.
- Machine Type: Must be left blank, empty, or null.

The COMPLEX Tier (Logical OR / Tripwires):
To be classified as COMPLEX, a job only needs to exceed ANY SINGLE upper threshold. Hitting just one of these wires instantly flags the job as Complex:
- CPU: Greater than 4000 mCPU.
- Memory: Greater than 8192 MiB.
- Duration: Greater than 60 minutes (1 hour).
- Machine Type: If a specific Machine Type is explicitly requested (not blank), ignore CPU/Memory math and immediately force the job to be COMPLEX.

The MEDIUM Tier (Fallback — treated as SIMPLE):
If a job is rejected from the SIMPLE tier (because it is too large) but does not trip any of the wires in the COMPLEX tier, classify it as MEDIUM. The system will treat MEDIUM the same as SIMPLE and route it to Cloud Run Jobs.

You must output your decision strictly as a valid JSON object. Do not include markdown formatting, backticks, or any conversational text. Use this exact schema:
{"complexity": "SIMPLE | MEDIUM | COMPLEX", "reason": "Provide a brief, 1-sentence explanation of which rules were evaluated to reach this decision."}`

// GeminiClassification is the JSON response returned by Gemini.
type GeminiClassification struct {
	Complexity string `json:"complexity"`
	Reason     string `json:"reason"`
}

// ClassifyWithGemini sends the job parameters to Gemini and returns
// the AI-determined complexity classification.
// apiKey should be passed from the GEMINI_API_KEY environment variable.
func ClassifyWithGemini(ctx context.Context, apiKey string, cpuMillis, memoryMiB, durationSec int64, machineType string) (*GeminiClassification, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	durationMin := durationSec / 60
	machineTypeStr := machineType
	if machineTypeStr == "" {
		machineTypeStr = `""`
	}

	prompt := fmt.Sprintf(
		`Evaluate this job -> CPU: %d mCPU, Memory: %d MiB, Duration: %d minutes, Machine Type: %s`,
		cpuMillis, memoryMiB, durationMin, machineTypeStr,
	)

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(prompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(geminiSystemInstruction, genai.RoleUser),
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}

	rawText := result.Text()

	var classification GeminiClassification
	if err := json.Unmarshal([]byte(rawText), &classification); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response %q: %w", rawText, err)
	}

	return &classification, nil
}
