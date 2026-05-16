package baseline

import (
	"math"
	"testing"
)

func TestComputeSessionDrift(t *testing.T) {
	tests := []struct {
		name     string
		actual   []int32
		baseline []int32
		want     float64
	}{
		{
			name:     "both empty returns 0.0",
			actual:   []int32{},
			baseline: []int32{},
			want:     0.0,
		},
		{
			name:     "identical sequences returns 0.0",
			actual:   []int32{1, 2, 3},
			baseline: []int32{1, 2, 3},
			want:     0.0,
		},
		{
			name:     "actual empty returns 1.0",
			actual:   []int32{},
			baseline: []int32{1, 2, 3},
			want:     1.0,
		},
		{
			name:     "baseline empty returns 1.0",
			actual:   []int32{1, 2, 3},
			baseline: []int32{},
			want:     1.0,
		},
		{
			name:     "single substitution returns 1/3",
			actual:   []int32{1, 2, 3},
			baseline: []int32{1, 2, 4},
			want:     1.0 / 3.0,
		},
		{
			name:     "single deletion returns 1/4",
			actual:   []int32{1, 2, 3, 4},
			baseline: []int32{1, 2, 3},
			want:     1.0 / 4.0,
		},
		{
			name:     "completely different sequences returns 1.0",
			actual:   []int32{1, 2, 3},
			baseline: []int32{4, 5, 6},
			want:     1.0,
		},
		{
			name:     "result never exceeds 1.0",
			actual:   []int32{1},
			baseline: []int32{2, 3, 4, 5},
			want:     1.0,
		},
		{
			name:     "single element identical",
			actual:   []int32{7},
			baseline: []int32{7},
			want:     0.0,
		},
		{
			name:     "single element different",
			actual:   []int32{7},
			baseline: []int32{8},
			want:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeSessionDrift(tt.actual, tt.baseline)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("ComputeSessionDrift(%v, %v) = %v, want %v", tt.actual, tt.baseline, got, tt.want)
			}
			// Invariant: result is always in [0.0, 1.0]
			if got < 0.0 || got > 1.0 {
				t.Errorf("ComputeSessionDrift result %v is outside [0.0, 1.0]", got)
			}
		})
	}
}
