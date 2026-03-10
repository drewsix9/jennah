package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
)

type objectSizeResolver func(ctx context.Context, inputPath string) (int64, error)

func cloneEnvVars(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ensureDistributedInputDataSize resolves INPUT_DATA_SIZE for BYTE_RANGE DWP jobs
// when the frontend omits it. For gs:// paths, it looks up object metadata and
// writes the resolved size back into env vars.
func ensureDistributedInputDataSize(ctx context.Context, envVars map[string]string, resolver objectSizeResolver) error {
	if envVars == nil {
		return nil
	}
	if resolver == nil {
		return fmt.Errorf("input size resolver is required")
	}

	enabledRaw, enabled, _ := lookupEnvVarCI(envVars, "ENABLE_DISTRIBUTED_MODE")
	if !enabled || !isTruthyValue(enabledRaw) {
		return nil
	}

	// RECORD mode preserves semantic records and does not require byte-size splits.
	modeRaw, hasMode, _ := lookupEnvVarCI(envVars, "DISTRIBUTION_MODE")
	if hasMode && strings.EqualFold(strings.TrimSpace(modeRaw), "RECORD") {
		return nil
	}

	sizeRaw, hasSize, sizeKey := lookupEnvVarCI(envVars, "INPUT_DATA_SIZE")
	if hasSize {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(sizeRaw), 10, 64); err == nil && parsed > 0 {
			return nil
		}
	}

	inputPath, hasPath, _ := lookupEnvVarCI(envVars, "INPUT_DATA_PATH")
	inputPath = strings.TrimSpace(inputPath)
	if !hasPath || inputPath == "" {
		return fmt.Errorf("INPUT_DATA_PATH is required when distributed mode is enabled and INPUT_DATA_SIZE is missing")
	}

	if !isGCSPath(inputPath) {
		// Non-GCS paths may still be resolved at runtime inside the container.
		return nil
	}

	size, err := resolver(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("failed to auto-resolve INPUT_DATA_SIZE from %q: %w", inputPath, err)
	}
	if size <= 0 {
		return fmt.Errorf("resolved INPUT_DATA_SIZE must be > 0, got %d", size)
	}

	if sizeKey == "" {
		sizeKey = "INPUT_DATA_SIZE"
	}
	envVars[sizeKey] = strconv.FormatInt(size, 10)
	return nil
}

func lookupEnvVarCI(envVars map[string]string, key string) (value string, found bool, actualKey string) {
	for k, v := range envVars {
		if strings.EqualFold(k, key) {
			return v, true, k
		}
	}
	return "", false, ""
}

func isTruthyValue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isGCSPath(path string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(path)), "gs://")
}

func getGCSObjectSize(ctx context.Context, gcsPath string) (int64, error) {
	bucket, object, err := parseGCSPath(gcsPath)
	if err != nil {
		return 0, err
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("create GCS client: %w", err)
	}
	defer client.Close()

	attrs, err := client.Bucket(bucket).Object(object).Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("get object attrs: %w", err)
	}
	return attrs.Size, nil
}

func parseGCSPath(path string) (bucket string, object string, err error) {
	p := strings.TrimSpace(path)
	if !isGCSPath(p) {
		return "", "", fmt.Errorf("invalid GCS path %q: must start with gs://", path)
	}
	p = p[5:]
	parts := strings.SplitN(p, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GCS path %q: expected gs://bucket/object", path)
	}
	bucket = strings.TrimSpace(parts[0])
	object = strings.TrimSpace(parts[1])
	if bucket == "" || object == "" {
		return "", "", fmt.Errorf("invalid GCS path %q: bucket/object must not be empty", path)
	}
	return bucket, object, nil
}
