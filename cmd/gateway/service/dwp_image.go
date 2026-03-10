package service

import (
	"fmt"
	"strings"
)

const DefaultDWPImageURI = "us-central1-docker.pkg.dev/labs-169405/demo-job-repo/demo-job:latest"

func resolveSubmittedImageURI(imageURI string, envVars map[string]string, defaultDWPImageURI string) (string, error) {
	if !isDistributedModeEnabled(envVars) {
		trimmedImageURI := strings.TrimSpace(imageURI)
		if trimmedImageURI != "" {
			return trimmedImageURI, nil
		}
		return "", fmt.Errorf("image_uri is required")
	}

	fallbackImageURI := strings.TrimSpace(defaultDWPImageURI)
	if fallbackImageURI == "" {
		return "", fmt.Errorf("image_uri is required for distributed jobs unless DEFAULT_DWP_IMAGE_URI is configured")
	}

	return fallbackImageURI, nil
}

func isDistributedModeEnabled(envVars map[string]string) bool {
	for key, value := range envVars {
		if !strings.EqualFold(key, "ENABLE_DISTRIBUTED_MODE") {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "y", "on":
			return true
		default:
			return false
		}
	}

	return false
}
