package drift

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/driftctl/driftctl/internal/model"
)

// Engine compares expected and actual resource sets.
type Engine struct {
	IgnoreTags       map[string]bool
	IgnoreAttributes map[string]bool
}

func NewEngine(cfg model.CompareConfig) *Engine {
	e := &Engine{
		IgnoreTags:       make(map[string]bool),
		IgnoreAttributes: make(map[string]bool),
	}
	for _, t := range cfg.IgnoreTags {
		e.IgnoreTags[t] = true
	}
	for _, a := range cfg.IgnoreAttributes {
		e.IgnoreAttributes[a] = true
	}
	// Always ignore terraform-managed tags by default
	for _, t := range []string{"terraform", "driftctl_scan"} {
		e.IgnoreTags[t] = true
	}
	return e
}

// Compare produces a drift report from expected (state) and actual (cloud) resources.
func (e *Engine) Compare(expected, actual []model.Resource) []model.DriftFinding {
	// Optimization 4: Use pointers to avoid copying large resource structs
	actualIndexByID := make(map[string]*model.Resource, len(actual))
	for i := range actual {
		actualIndexByID[actual[i].ID] = &actual[i]
	}

	expectedIndexByID := make(map[string]*model.Resource, len(expected))
	var findings []model.DriftFinding

	// Check for missing resources and attribute/tag changes
	for i := range expected {
		exp := &expected[i]
		expectedIndexByID[exp.ID] = exp
		act, ok := actualIndexByID[exp.ID]
		if !ok {
			findings = append(findings, model.DriftFinding{
				Kind:         model.DriftMissingInCloud,
				ResourceID:   exp.ID,
				ResourceType: exp.Type,
				ResourceName: exp.Name,
				Severity:     severityForMissing(*exp),
			})
			continue
		}
		findings = append(findings, e.diffAttributes(*exp, *act)...)
		findings = append(findings, e.diffTags(*exp, *act)...)
	}

	// Check for extra resources
	for i := range actual {
		act := &actual[i]
		if _, ok := expectedIndexByID[act.ID]; !ok {
			findings = append(findings, model.DriftFinding{
				Kind:         model.DriftExtraInCloud,
				ResourceID:   act.ID,
				ResourceType: act.Type,
				ResourceName: act.Name,
				Severity:     model.SeverityWarning,
			})
		}
	}

	return findings
}

func (e *Engine) diffAttributes(exp, act model.Resource) []model.DriftFinding {
	var findings []model.DriftFinding
	allKeys := make(map[string]bool)
	for k := range exp.Attributes {
		allKeys[k] = true
	}
	for k := range act.Attributes {
		allKeys[k] = true
	}

	for key := range allKeys {
		if e.IgnoreAttributes[key] {
			continue
		}
		ev := exp.Attributes[key]
		av := act.Attributes[key]
		// Optimization 1: Direct value comparison instead of JSON serialization
		if valuesEqualDirect(ev, av) {
			continue
		}
		findings = append(findings, model.DriftFinding{
			Kind:         model.DriftAttributeChange,
			ResourceID:   exp.ID,
			ResourceType: exp.Type,
			ResourceName: exp.Name,
			Field:        key,
			Expected:     ev,
			Actual:       av,
			Severity:     model.SeverityWarning,
		})
	}
	return findings
}

// Optimization 3: Single-pass tag comparison without intermediate maps
func (e *Engine) diffTags(exp, act model.Resource) []model.DriftFinding {
	var findings []model.DriftFinding

	// Create filtered views without allocating new maps
	for key, expVal := range exp.Tags {
		if e.IgnoreTags[key] {
			continue
		}
		actVal, ok := act.Tags[key]
		if !ok {
			// Tag missing in actual
			findings = append(findings, model.DriftFinding{
				Kind:         model.DriftTagsChanged,
				ResourceID:   exp.ID,
				ResourceType: exp.Type,
				ResourceName: exp.Name,
				Field:        fmt.Sprintf("tags.%s", key),
				Expected:     expVal,
				Actual:       nil,
				Severity:     model.SeverityInfo,
			})
			continue
		}
		if expVal != actVal {
			// Tag value changed
			findings = append(findings, model.DriftFinding{
				Kind:         model.DriftTagsChanged,
				ResourceID:   exp.ID,
				ResourceType: exp.Type,
				ResourceName: exp.Name,
				Field:        fmt.Sprintf("tags.%s", key),
				Expected:     expVal,
				Actual:       actVal,
				Severity:     model.SeverityInfo,
			})
		}
	}

	// Check for extra tags in actual
	for key, actVal := range act.Tags {
		if e.IgnoreTags[key] {
			continue
		}
		if _, ok := exp.Tags[key]; !ok {
			// Extra tag in actual
			findings = append(findings, model.DriftFinding{
				Kind:         model.DriftTagsChanged,
				ResourceID:   exp.ID,
				ResourceType: exp.Type,
				ResourceName: exp.Name,
				Field:        fmt.Sprintf("tags.%s", key),
				Expected:     nil,
				Actual:       actVal,
				Severity:     model.SeverityInfo,
			})
		}
	}

	return findings
}

// Optimization 1: Direct value comparison without JSON serialization
// Compares two values with normalization for numeric types and nested structures
func valuesEqualDirect(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Fast path for simple types
	if av, ok := a.(string); ok {
		if bv, ok := b.(string); ok {
			return av == bv
		}
	}

	// Normalize and compare complex types
	return valuesEqualNormalized(normalizeForCompare(a), normalizeForCompare(b))
}

// Deep equality comparison for normalized values
func valuesEqualNormalized(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, aval := range av {
			bval, ok := bv[k]
			if !ok || !valuesEqualNormalized(aval, bval) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !valuesEqualNormalized(av[i], bv[i]) {
				return false
			}
		}
		return true
	case int64:
		if bv, ok := b.(int64); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	}

	return reflect.DeepEqual(a, b)
}

func normalizeForCompare(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = normalizeForCompare(item)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeForCompare(item)
		}
		return out
	case float64:
		if t == float64(int64(t)) {
			return int64(t)
		}
		return t
	default:
		return v
	}
}

func severityForMissing(r model.Resource) string {
	if env, ok := r.Tags["env"]; ok {
		if strings.EqualFold(env, "prod") || strings.EqualFold(env, "production") {
			return model.SeverityCritical
		}
	}
	return model.SeverityCritical
}

// BuildSummary computes summary statistics from findings and resource counts.
func BuildSummary(expectedCount int, findings []model.DriftFinding) model.DriftSummary {
	s := model.DriftSummary{
		TotalResources: expectedCount,
		TotalFindings:  len(findings),
	}
	for _, f := range findings {
		switch f.Kind {
		case model.DriftMissingInCloud:
			s.MissingInCloud++
		case model.DriftExtraInCloud:
			s.ExtraInCloud++
		case model.DriftAttributeChange:
			s.AttributeChanges++
		case model.DriftTagsChanged:
			s.TagChanges++
		}
	}
	return s
}
