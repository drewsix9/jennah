package demo

import (
	"fmt"
)

// ByteRange represents a chunk of bytes to process
type ByteRange struct {
	StartByte      int64
	EndByte        int64 // inclusive
	InstanceID     int
	TotalInstances int
}

// ChunkCalculator divides file processing work across instances
type ChunkCalculator struct {
	fileSize       int64
	totalInstances int
}

// NewChunkCalculator creates a calculator for dividing file work
func NewChunkCalculator(fileSize int64, totalInstances int) *ChunkCalculator {
	return &ChunkCalculator{
		fileSize:       fileSize,
		totalInstances: totalInstances,
	}
}

// Calculate computes the byte range for a given instance
// Returns (startByte, endByte) inclusive for this instance
func (c *ChunkCalculator) Calculate(instanceID int) (*ByteRange, error) {
	// Validation
	if instanceID < 0 {
		return nil, fmt.Errorf("instanceID must be >= 0, got %d", instanceID)
	}
	if instanceID >= c.totalInstances {
		return nil, fmt.Errorf("instanceID %d >= totalInstances %d", instanceID, c.totalInstances)
	}
	if c.fileSize < 0 {
		return nil, fmt.Errorf("fileSize cannot be negative")
	}
	if c.totalInstances < 1 {
		return nil, fmt.Errorf("totalInstances must be >= 1")
	}

	// Edge case: file smaller than number of instances
	if c.fileSize < int64(c.totalInstances) {
		// Only first N instances get work (1 byte each)
		if instanceID < int(c.fileSize) {
			return &ByteRange{
				StartByte:      int64(instanceID),
				EndByte:        int64(instanceID), // 1 byte
				InstanceID:     instanceID,
				TotalInstances: c.totalInstances,
			}, nil
		}
		// Remaining instances get empty range
		return &ByteRange{
			StartByte:      0,
			EndByte:        -1, // Empty range marker
			InstanceID:     instanceID,
			TotalInstances: c.totalInstances,
		}, nil
	}

	// Normal case: divide evenly
	bytesPerInstance := c.fileSize / int64(c.totalInstances)
	remainder := c.fileSize % int64(c.totalInstances)

	var startByte int64
	if instanceID == 0 {
		startByte = 0
	} else {
		startByte = int64(instanceID)*bytesPerInstance + min(int64(instanceID), remainder)
	}

	var endByte int64
	if instanceID == c.totalInstances-1 {
		// Last instance gets remaining bytes
		endByte = c.fileSize - 1
	} else {
		endByte = int64(instanceID+1)*bytesPerInstance + min(int64(instanceID+1), remainder) - 1
	}

	return &ByteRange{
		StartByte:      startByte,
		EndByte:        endByte,
		InstanceID:     instanceID,
		TotalInstances: c.totalInstances,
	}, nil
}

// Size returns the number of bytes in the range
func (b *ByteRange) Size() int64 {
	if b.EndByte < b.StartByte {
		return 0 // Empty range
	}
	return b.EndByte - b.StartByte + 1
}

// IsEmpty returns true if range has no bytes
func (b *ByteRange) IsEmpty() bool {
	return b.EndByte < b.StartByte
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
