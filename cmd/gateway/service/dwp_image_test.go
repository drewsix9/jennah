package service

import "testing"

func TestResolveSubmittedImageURI(t *testing.T) {
	t.Run("uses submitted image for non-distributed jobs", func(t *testing.T) {
		resolved, err := resolveSubmittedImageURI("gcr.io/project/custom:latest", nil, DefaultDWPImageURI)
		if err != nil {
			t.Fatalf("resolveSubmittedImageURI returned error: %v", err)
		}
		if resolved != "gcr.io/project/custom:latest" {
			t.Fatalf("resolveSubmittedImageURI = %q, want submitted image", resolved)
		}
	})

	t.Run("falls back to default image for distributed jobs", func(t *testing.T) {
		resolved, err := resolveSubmittedImageURI("", map[string]string{
			"ENABLE_DISTRIBUTED_MODE": "true",
		}, DefaultDWPImageURI)
		if err != nil {
			t.Fatalf("resolveSubmittedImageURI returned error: %v", err)
		}
		if resolved != DefaultDWPImageURI {
			t.Fatalf("resolveSubmittedImageURI = %q, want %q", resolved, DefaultDWPImageURI)
		}
	})

	t.Run("overrides submitted image for distributed jobs", func(t *testing.T) {
		resolved, err := resolveSubmittedImageURI("gcr.io/project/custom:latest", map[string]string{
			"ENABLE_DISTRIBUTED_MODE": "true",
		}, DefaultDWPImageURI)
		if err != nil {
			t.Fatalf("resolveSubmittedImageURI returned error: %v", err)
		}
		if resolved != DefaultDWPImageURI {
			t.Fatalf("resolveSubmittedImageURI = %q, want %q", resolved, DefaultDWPImageURI)
		}
	})

	t.Run("rejects blank image for non-distributed jobs", func(t *testing.T) {
		if _, err := resolveSubmittedImageURI("", nil, DefaultDWPImageURI); err == nil {
			t.Fatal("resolveSubmittedImageURI returned nil error, want error")
		}
	})

	t.Run("rejects distributed jobs without any configured fallback image", func(t *testing.T) {
		if _, err := resolveSubmittedImageURI("", map[string]string{
			"ENABLE_DISTRIBUTED_MODE": "true",
		}, ""); err == nil {
			t.Fatal("resolveSubmittedImageURI returned nil error, want error")
		}
	})
}
