package analyzer

import "testing"

func TestIsStringType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"string", true},
		{"int", false},
		{"float64", false},
		{"bool", false},
		{"[]string", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isStringType(tt.typeName)
			if result != tt.expected {
				t.Errorf("isStringType(%q): expected %v, got %v", tt.typeName, tt.expected, result)
			}
		})
	}
}

func TestIsNumericType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"int", true},
		{"int8", true},
		{"int16", true},
		{"int32", true},
		{"int64", true},
		{"uint", true},
		{"uint8", true},
		{"uint16", true},
		{"uint32", true},
		{"uint64", true},
		{"float32", true},
		{"float64", true},
		{"string", false},
		{"bool", false},
		{"[]int", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isNumericType(tt.typeName)
			if result != tt.expected {
				t.Errorf("isNumericType(%q): expected %v, got %v", tt.typeName, tt.expected, result)
			}
		})
	}
}
