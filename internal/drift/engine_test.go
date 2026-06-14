package drift

import (
	"testing"

	"github.com/driftctl/driftctl/internal/model"
)

func TestValuesEqualDirect(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        "test",
			b:        nil,
			expected: false,
		},
		{
			name:     "equal strings",
			a:        "test",
			b:        "test",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "test1",
			b:        "test2",
			expected: false,
		},
		{
			name:     "equal maps",
			a:        map[string]any{"key": "value"},
			b:        map[string]any{"key": "value"},
			expected: true,
		},
		{
			name:     "different maps",
			a:        map[string]any{"key": "value1"},
			b:        map[string]any{"key": "value2"},
			expected: false,
		},
		{
			name:     "equal arrays",
			a:        []any{1, 2, 3},
			b:        []any{1, 2, 3},
			expected: true,
		},
		{
			name:     "different arrays",
			a:        []any{1, 2, 3},
			b:        []any{1, 2, 4},
			expected: false,
		},
		{
			name:     "float to int normalization",
			a:        float64(42),
			b:        int64(42),
			expected: true,
		},
		{
			name:     "nested equal structures",
			a:        map[string]any{"arr": []any{1, 2}, "key": "val"},
			b:        map[string]any{"arr": []any{1, 2}, "key": "val"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valuesEqualDirect(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("valuesEqualDirect(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestCompareMissingResources(t *testing.T) {
	expected := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Name:       "web-server",
			Attributes: map[string]any{"instance_type": "t2.micro"},
			Tags:       map[string]string{"env": "prod"},
		},
	}
	actual := []model.Resource{}

	eng := NewEngine(model.CompareConfig{})
	findings := eng.Compare(expected, actual)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Kind != model.DriftMissingInCloud {
		t.Errorf("expected DriftMissingInCloud, got %s", findings[0].Kind)
	}
}

func TestCompareExtraResources(t *testing.T) {
	expected := []model.Resource{}
	actual := []model.Resource{
		{
			ID:         "aws/instance/i-456",
			Type:       "aws_instance",
			CloudID:    "i-456",
			Name:       "orphan-server",
			Attributes: map[string]any{},
			Tags:       map[string]string{},
		},
	}

	eng := NewEngine(model.CompareConfig{})
	findings := eng.Compare(expected, actual)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Kind != model.DriftExtraInCloud {
		t.Errorf("expected DriftExtraInCloud, got %s", findings[0].Kind)
	}
}

func TestCompareAttributeChanges(t *testing.T) {
	expected := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Name:       "web-server",
			Attributes: map[string]any{"instance_type": "t2.micro"},
			Tags:       map[string]string{},
		},
	}
	actual := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Name:       "web-server",
			Attributes: map[string]any{"instance_type": "t2.small"}, // Changed
			Tags:       map[string]string{},
		},
	}

	eng := NewEngine(model.CompareConfig{})
	findings := eng.Compare(expected, actual)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Kind != model.DriftAttributeChange {
		t.Errorf("expected DriftAttributeChange, got %s", findings[0].Kind)
	}
}

func TestCompareTags(t *testing.T) {
	expected := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Attributes: map[string]any{},
			Tags:       map[string]string{"env": "prod", "team": "backend"},
		},
	}
	actual := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Attributes: map[string]any{},
			Tags:       map[string]string{"env": "dev"}, // env changed, team missing
		},
	}

	eng := NewEngine(model.CompareConfig{})
	findings := eng.Compare(expected, actual)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	for _, f := range findings {
		if f.Kind != model.DriftTagsChanged {
			t.Errorf("expected DriftTagsChanged, got %s", f.Kind)
		}
	}
}

func TestIgnoreTags(t *testing.T) {
	expected := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Attributes: map[string]any{},
			Tags:       map[string]string{"env": "prod", "ignored": "value1"},
		},
	}
	actual := []model.Resource{
		{
			ID:         "aws/instance/i-123",
			Type:       "aws_instance",
			CloudID:    "i-123",
			Attributes: map[string]any{},
			Tags:       map[string]string{"env": "prod", "ignored": "value2"},
		},
	}

	eng := NewEngine(model.CompareConfig{
		IgnoreTags: []string{"ignored"},
	})
	findings := eng.Compare(expected, actual)

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with ignored tag, got %d", len(findings))
	}
}

// Benchmark tests
func BenchmarkCompareSmall(b *testing.B) {
	expected := generateResources(10)
	actual := generateResources(10)
	eng := NewEngine(model.CompareConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Compare(expected, actual)
	}
}

func BenchmarkCompareLarge(b *testing.B) {
	expected := generateResources(1000)
	actual := generateResources(1000)
	eng := NewEngine(model.CompareConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Compare(expected, actual)
	}
}

func BenchmarkCompareMassive(b *testing.B) {
	expected := generateResources(5000)
	actual := generateResources(5000)
	eng := NewEngine(model.CompareConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Compare(expected, actual)
	}
}

func generateResources(count int) []model.Resource {
	resources := make([]model.Resource, count)
	for i := 0; i < count; i++ {
		id := string(rune(i))
		resources[i] = model.Resource{
			ID:      "aws/instance/" + id,
			Type:    "aws_instance",
			CloudID: id,
			Name:    "instance-" + id,
			Attributes: map[string]any{
				"instance_type": "t2.micro",
				"ami":           "ami-12345",
				"vpc_id":        "vpc-123",
			},
			Tags: map[string]string{
				"env":  "prod",
				"team": "backend",
				"app":  "api",
			},
		}
	}
	return resources
}
