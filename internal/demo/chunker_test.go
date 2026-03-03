package demo

import (
	"testing"
)

func TestCalculateByteRange_Normal(t *testing.T) {
	// 1000 bytes, 4 instances
	calc := NewChunkCalculator(1000, 4)

	tests := []struct {
		instanceID    int
		expectedStart int64
		expectedEnd   int64
		expectedSize  int64
	}{
		{0, 0, 249, 250},
		{1, 250, 499, 250},
		{2, 500, 749, 250},
		{3, 750, 999, 250},
	}

	for _, tt := range tests {
		br, err := calc.Calculate(tt.instanceID)
		if err != nil {
			t.Fatalf("instance %d: unexpected error: %v", tt.instanceID, err)
		}
		if br.StartByte != tt.expectedStart {
			t.Errorf("instance %d: start %d, want %d", tt.instanceID, br.StartByte, tt.expectedStart)
		}
		if br.EndByte != tt.expectedEnd {
			t.Errorf("instance %d: end %d, want %d", tt.instanceID, br.EndByte, tt.expectedEnd)
		}
		if br.Size() != tt.expectedSize {
			t.Errorf("instance %d: size %d, want %d", tt.instanceID, br.Size(), tt.expectedSize)
		}
	}
}

func TestCalculateByteRange_Uneven(t *testing.T) {
	// 1001 bytes, 4 instances (remainder = 1)
	// First instance gets extra byte
	calc := NewChunkCalculator(1001, 4)

	br0, _ := calc.Calculate(0)
	br1, _ := calc.Calculate(1)
	br2, _ := calc.Calculate(2)
	br3, _ := calc.Calculate(3)

	if br0.Size() != 251 { // 250 + 1 remainder
		t.Errorf("instance 0: size %d, want 251", br0.Size())
	}
	if br1.Size() != 250 {
		t.Errorf("instance 1: size %d, want 250", br1.Size())
	}
	if br2.Size() != 250 {
		t.Errorf("instance 2: size %d, want 250", br2.Size())
	}
	if br3.Size() != 250 {
		t.Errorf("instance 3: size %d, want 250", br3.Size())
	}
}

func TestCalculateByteRange_FileSmall(t *testing.T) {
	// 2 bytes, 4 instances
	calc := NewChunkCalculator(2, 4)

	br0, _ := calc.Calculate(0)
	br1, _ := calc.Calculate(1)
	br2, _ := calc.Calculate(2)
	br3, _ := calc.Calculate(3)

	if br0.Size() != 1 || br1.Size() != 1 {
		t.Errorf("instances 0-1 should get 1 byte each")
	}
	if !br2.IsEmpty() || !br3.IsEmpty() {
		t.Errorf("instances 2-3 should be empty")
	}
}

func TestCalculateByteRange_EmptyFile(t *testing.T) {
	// 0 bytes, 4 instances
	calc := NewChunkCalculator(0, 4)

	for i := 0; i < 4; i++ {
		br, _ := calc.Calculate(i)
		if !br.IsEmpty() {
			t.Errorf("instance %d: should be empty for 0-byte file", i)
		}
	}
}

func TestCalculateByteRange_SingleInstance(t *testing.T) {
	// 1000 bytes, 1 instance
	calc := NewChunkCalculator(1000, 1)

	br0, _ := calc.Calculate(0)
	if br0.StartByte != 0 || br0.EndByte != 999 || br0.Size() != 1000 {
		t.Errorf("single instance should process entire file")
	}
}

func TestCalculateByteRange_Error_InvalidInstanceID(t *testing.T) {
	calc := NewChunkCalculator(1000, 4)

	_, err := calc.Calculate(4) // Out of range
	if err == nil {
		t.Errorf("should error for instanceID >= totalInstances")
	}
}
