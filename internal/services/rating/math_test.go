package rating

import (
	"math"
	"testing"
)

func TestCAGR(t *testing.T) {
	tests := []struct {
		name   string
		start  float64
		end    float64
		years  float64
		want   float64
		margin float64 // acceptable error margin
	}{
		{
			name:   "100% growth over 1 year",
			start:  100,
			end:    200,
			years:  1,
			want:   1.0, // 100% CAGR
			margin: 0.001,
		},
		{
			name:   "no growth",
			start:  100,
			end:    100,
			years:  3,
			want:   0, // 0% CAGR
			margin: 0.001,
		},
		{
			name:   "10% annual growth over 3 years",
			start:  100,
			end:    133.1,
			years:  3,
			want:   0.1, // ~10% CAGR
			margin: 0.01,
		},
		{
			name:   "zero start returns zero",
			start:  0,
			end:    100,
			years:  1,
			want:   0,
			margin: 0.001,
		},
		{
			name:   "zero years returns zero",
			start:  100,
			end:    200,
			years:  0,
			want:   0,
			margin: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CAGR(tt.start, tt.end, tt.years)
			if math.Abs(got-tt.want) > tt.margin {
				t.Errorf("CAGR(%f, %f, %f) = %f, want %f (±%f)", tt.start, tt.end, tt.years, got, tt.want, tt.margin)
			}
		})
	}
}

func TestStddev(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
		margin float64
	}{
		{
			name:   "empty slice returns zero",
			values: []float64{},
			want:   0,
			margin: 0.001,
		},
		{
			name:   "single value returns zero",
			values: []float64{5.0},
			want:   0,
			margin: 0.001,
		},
		{
			name:   "known stddev",
			values: []float64{2, 4, 4, 4, 5, 5, 7, 9},
			want:   2.0, // Known population stddev
			margin: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Stddev(tt.values)
			if math.Abs(got-tt.want) > tt.margin {
				t.Errorf("Stddev() = %f, want %f (±%f)", got, tt.want, tt.margin)
			}
		})
	}
}

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		min   float64
		max   float64
		want  float64
	}{
		{"value below min", -5, 0, 100, 0},
		{"value above max", 150, 0, 100, 100},
		{"value in range", 50, 0, 100, 50},
		{"value at min", 0, 0, 100, 0},
		{"value at max", 100, 0, 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampFloat64(tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("ClampFloat64(%f, %f, %f) = %f, want %f", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}
