package state

import (
	"testing"
)

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "float to int",
			input:    float64(42),
			expected: int64(42),
		},
		{
			name:     "float with decimal",
			input:    float64(42.5),
			expected: float64(42.5),
		},
		{
			name:     "string passthrough",
			input:    "test",
			expected: "test",
		},
		{
			name:     "empty array",
			input:    []any{},
			expected: []any{},
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeValue(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeValue(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkNormalizeSmall(b *testing.B) {
	data := map[string]any{
		"instance_type": float64(2),
		"count":         float64(5),
		"tags": map[string]any{
			"env": "prod",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeValue(data)
	}
}

func BenchmarkNormalizeLarge(b *testing.B) {
	data := make(map[string]any)
	for i := 0; i < 1000; i++ {
		data[string(rune(i))] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeValue(data)
	}
}
