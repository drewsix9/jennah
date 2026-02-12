package database

import "time"

// Tenant represents an organization/team using the platform
type Tenant struct {
	TenantId  string    `spanner:"TenantId"`
	Name      string    `spanner:"Name"`
	CreatedAt time.Time `spanner:"CreatedAt"`
	UpdatedAt time.Time `spanner:"UpdatedAt"`
}

// Job represents a deployment job
type Job struct {
	TenantId        string    `spanner:"TenantId"`
	JobId           string    `spanner:"JobId"`
	Status          string    `spanner:"Status"`
	ImageUri        string    `spanner:"ImageUri"`
	Commands        []string  `spanner:"Commands"`
	CreatedAt       time.Time `spanner:"CreatedAt"`
	UpdatedAt       time.Time `spanner:"UpdatedAt"`
	CompletedAt     time.Time `spanner:"CompletedAt"`
	ErrorMessage    string    `spanner:"ErrorMessage"`
	GcpBatchJobName string    `spanner:"GcpBatchJobName"` // Full GCP resource name
}

// JobStatus constants
const (
	JobStatusPending   = "PENDING"
	JobStatusRunning   = "RUNNING"
	JobStatusCompleted = "COMPLETED"
	JobStatusFailed    = "FAILED"
)
