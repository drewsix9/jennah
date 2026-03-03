package demo

import (
	"fmt"
	"log"
	"time"
)

// Error categories
const (
	ErrFileNotFound     = "FILE_NOT_FOUND"
	ErrPermissionDenied = "PERMISSION_DENIED"
	ErrNetworkTimeout   = "NETWORK_TIMEOUT"
	ErrInvalidConfig    = "INVALID_CONFIG"
	ErrProcessing       = "PROCESSING_ERROR"
)

// RetryStrategy defines retry behavior
type RetryStrategy struct {
	MaxRetries int
	Delays     []time.Duration
}

// DefaultRetry provides standard retry configuration
func DefaultRetry() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries: 3,
		Delays: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
		},
	}
}

// ErrorHandler manages error handling and retries
type ErrorHandler struct {
	strategy *RetryStrategy
}

// NewErrorHandler creates error handler with default retry strategy
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		strategy: DefaultRetry(),
	}
}

// HandleWithRetry executes operation with exponential backoff retry
func (eh *ErrorHandler) HandleWithRetry(op func() error, opName string) error {
	var lastErr error

	for attempt := 0; attempt <= eh.strategy.MaxRetries; attempt++ {
		err := op()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry after final attempt
		if attempt >= eh.strategy.MaxRetries {
			return fmt.Errorf("%s failed after %d retries: %w", opName, eh.strategy.MaxRetries, lastErr)
		}

		// Wait before retrying
		delay := eh.strategy.Delays[attempt]
		log.Printf("  Attempt %d failed: %v (retrying in %v)", attempt+1, err, delay)
		time.Sleep(delay)
	}

	return lastErr
}

// HandleError logs error appropriately based on severity
func (eh *ErrorHandler) HandleError(errType string, err error) {
	switch errType {
	case ErrFileNotFound:
		log.Printf("ERROR: Input file not found - check INPUT_DATA_PATH: %v", err)
	case ErrPermissionDenied:
		log.Printf("ERROR: Permission denied - check GCS credentials and IAM roles: %v", err)
	case ErrNetworkTimeout:
		log.Printf("ERROR: Network timeout - will retry: %v", err)
	case ErrInvalidConfig:
		log.Fatalf("FATAL: Invalid configuration - cannot continue: %v", err)
	case ErrProcessing:
		log.Printf("ERROR: Processing error: %v", err)
	default:
		log.Printf("ERROR: %v", err)
	}
}
