package service

import (
	"context"
	"errors"
	"testing"
)

func TestEnsureDistributedInputDataSize_SkipsWhenDistributedDisabled(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "false",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
	}

	called := false
	resolver := func(context.Context, string) (int64, error) {
		called = true
		return 0, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("resolver should not be called when distributed mode is disabled")
	}
	if _, ok := env["INPUT_DATA_SIZE"]; ok {
		t.Fatalf("INPUT_DATA_SIZE should not be injected")
	}
}

func TestEnsureDistributedInputDataSize_SkipsForRecordMode(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "RECORD",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
	}

	called := false
	resolver := func(context.Context, string) (int64, error) {
		called = true
		return 0, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("resolver should not be called for RECORD mode")
	}
}

func TestEnsureDistributedInputDataSize_UsesExistingPositiveValue(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
		"INPUT_DATA_SIZE":         "12345",
	}

	called := false
	resolver := func(context.Context, string) (int64, error) {
		called = true
		return 999, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("resolver should not be called when INPUT_DATA_SIZE is already valid")
	}
	if got := env["INPUT_DATA_SIZE"]; got != "12345" {
		t.Fatalf("INPUT_DATA_SIZE changed unexpectedly: got %q", got)
	}
}

func TestEnsureDistributedInputDataSize_ResolvesFromGCSWhenMissing(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
	}

	called := 0
	resolver := func(_ context.Context, path string) (int64, error) {
		called++
		if path != "gs://bucket/input/data.txt" {
			t.Fatalf("unexpected path: %q", path)
		}
		return 6789, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("resolver call count: got %d, want 1", called)
	}
	if got := env["INPUT_DATA_SIZE"]; got != "6789" {
		t.Fatalf("INPUT_DATA_SIZE: got %q, want 6789", got)
	}
}

func TestEnsureDistributedInputDataSize_ReplacesInvalidValue(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
		"INPUT_DATA_SIZE":         "not-a-number",
	}

	resolver := func(context.Context, string) (int64, error) {
		return 4321, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := env["INPUT_DATA_SIZE"]; got != "4321" {
		t.Fatalf("INPUT_DATA_SIZE: got %q, want 4321", got)
	}
}

func TestEnsureDistributedInputDataSize_ReplacesZeroValue(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
		"INPUT_DATA_SIZE":         "0",
	}

	resolver := func(context.Context, string) (int64, error) {
		return 2468, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := env["INPUT_DATA_SIZE"]; got != "2468" {
		t.Fatalf("INPUT_DATA_SIZE: got %q, want 2468", got)
	}
}

func TestEnsureDistributedInputDataSize_LeavesNonGCSPathUntouched(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "/data/input.txt",
	}

	called := false
	resolver := func(context.Context, string) (int64, error) {
		called = true
		return 0, nil
	}

	if err := ensureDistributedInputDataSize(context.Background(), env, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("resolver should not be called for non-gcs paths")
	}
	if _, ok := env["INPUT_DATA_SIZE"]; ok {
		t.Fatalf("INPUT_DATA_SIZE should remain unset for non-gcs path")
	}
}

func TestEnsureDistributedInputDataSize_ErrorsWhenPathMissing(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
	}

	err := ensureDistributedInputDataSize(context.Background(), env, func(context.Context, string) (int64, error) {
		return 0, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnsureDistributedInputDataSize_ErrorsWhenResolverFails(t *testing.T) {
	env := map[string]string{
		"ENABLE_DISTRIBUTED_MODE": "true",
		"DISTRIBUTION_MODE":       "BYTE_RANGE",
		"INPUT_DATA_PATH":         "gs://bucket/input/data.txt",
	}

	wantErr := errors.New("metadata denied")
	err := ensureDistributedInputDataSize(context.Background(), env, func(context.Context, string) (int64, error) {
		return 0, wantErr
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped resolver error, got: %v", err)
	}
}
