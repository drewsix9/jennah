package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alphauslabs/jennah/internal/batch"
)

// JobConfigFile represents the structure of the job configuration JSON file.
type JobConfigFile struct {
	DefaultResources  ResourceProfile            `json:"defaultResources"`
	ResourceProfiles  map[string]ResourceProfile `json:"resourceProfiles"`
}

// ResourceProfile defines resource requirements for a job.
type ResourceProfile struct {
	CPUMillis             int64 `json:"cpuMillis"`
	MemoryMiB             int64 `json:"memoryMiB"`
	MaxRunDurationSeconds int64 `json:"maxRunDurationSeconds"`
}

// LoadJobConfig loads job configuration from a JSON file.
func LoadJobConfig(filePath string) (*JobConfigFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config JobConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

// GetResourceRequirements returns resource requirements for a profile name.
// If profileName is empty or not found, returns default resources.
func (c *JobConfigFile) GetResourceRequirements(profileName string) *batch.ResourceRequirements {
	var profile ResourceProfile

	if profileName != "" {
		if p, exists := c.ResourceProfiles[profileName]; exists {
			profile = p
		} else {
			profile = c.DefaultResources
		}
	} else {
		profile = c.DefaultResources
	}

	return &batch.ResourceRequirements{
		CPUMillis:             profile.CPUMillis,
		MemoryMiB:             profile.MemoryMiB,
		MaxRunDurationSeconds: profile.MaxRunDurationSeconds,
	}
}
