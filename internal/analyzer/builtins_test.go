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

// TestIsIntegerTypePredeclaredAliases locks Go's predeclared integer aliases
// into the integer set: byte is uint8 and rune is int32, so a named type over
// either must classify as `integer` rather than falling through to `object`.
// uintptr stays out — a machine address is not an API value (see the diagnostic).
func TestIsIntegerTypePredeclaredAliases(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"byte", true},
		{"rune", true},
		{"uintptr", false},
		{"complex64", false},
		{"error", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			if got := isIntegerType(tt.typeName); got != tt.expected {
				t.Errorf("isIntegerType(%q): expected %v, got %v", tt.typeName, tt.expected, got)
			}
			if got := isNumericType(tt.typeName); got != tt.expected {
				t.Errorf("isNumericType(%q): expected %v, got %v", tt.typeName, tt.expected, got)
			}
		})
	}
}
