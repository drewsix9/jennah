package demo

import (
	"testing"
)

func TestParseGCSPath_Valid(t *testing.T) {
	tests := []struct {
		path     string
		wantBucket string
		wantKey    string
	}{
		{
			path:       "gs://my-bucket/path/to/file.txt",
			wantBucket: "my-bucket",
			wantKey:    "path/to/file.txt",
		},
		{
			path:       "gs://bucket/file.txt",
			wantBucket: "bucket",
			wantKey:    "file.txt",
		},
		{
			path:       "gs://data/deeply/nested/path/input.json",
			wantBucket: "data",
			wantKey:    "deeply/nested/path/input.json",
		},
	}

	for _, tt := range tests {
		bucket, key, err := ParseGCSPath(tt.path)
		if err != nil {
			t.Errorf("ParseGCSPath(%q): unexpected error: %v", tt.path, err)
		}
		if bucket != tt.wantBucket {
			t.Errorf("ParseGCSPath(%q): bucket %q, want %q", tt.path, bucket, tt.wantBucket)
		}
		if key != tt.wantKey {
			t.Errorf("ParseGCSPath(%q): key %q, want %q", tt.path, key, tt.wantKey)
		}
	}
}

func TestParseGCSPath_Invalid(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{
			path: "/local/path/file.txt",
			want: "invalid GCS path: must start with gs://",
		},
		{
			path: "gs://",
			want: "invalid GCS path: must be gs://bucket/key",
		},
		{
			path: "gs://bucket-only",
			want: "invalid GCS path: must be gs://bucket/key",
		},
		{
			path: "http://example.com/file",
			want: "invalid GCS path: must start with gs://",
		},
	}

	for _, tt := range tests {
		_, _, err := ParseGCSPath(tt.path)
		if err == nil {
			t.Errorf("ParseGCSPath(%q): expected error, got nil", tt.path)
		}
	}
}

func TestIsGCSPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"gs://bucket/key", true},
		{"gs://my-bucket/path/to/file.txt", true},
		{"/local/path", false},
		{"http://example.com", false},
		{"./relative/path", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsGCSPath(tt.path)
		if got != tt.want {
			t.Errorf("IsGCSPath(%q): got %v, want %v", tt.path, got, tt.want)
		}
	}
}
