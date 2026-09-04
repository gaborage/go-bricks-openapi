package analyzer

// Go builtin type-name classification. Used by the Shape decoder to tell a
// primitive from a named type. The generator's constraint module keeps its own
// classification for OpenAPI-kind purposes; the two lists are the same lexical
// fact (Go's builtin numeric/string types) and change only when Go does.

const (
	// OpenAPI 3-way kind value (also a value of FieldInfo.UnderlyingKind).
	kindInteger = "integer"

	// Go primitive type names referenced for type discrimination
	goTypeInt64 = "int64"
	goTypeByte  = "byte"
	goTypeUint8 = "uint8"
	goTypeBool  = "bool"
	// unknownTypeName is the name the retired type-string renderer produced for
	// an AST node it did not model; embeddedFields still reports it in warnings.
	unknownTypeName = "unknown"
	goTypeAny       = "any"
	goTypeInterface = "interface{}"
	goTypeFloat32   = "float32"
	goTypeFloat64   = "float64"
)

// isStringType checks if the type is a string type
func isStringType(typeName string) bool {
	return typeName == goTypeString
}

// isIntegerType reports whether the Go type name is a signed/unsigned integer.
func isIntegerType(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", goTypeInt64,
		"uint", goTypeUint8, "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// isFloatType reports whether the Go type name is a floating-point type.
func isFloatType(typeName string) bool {
	return typeName == goTypeFloat32 || typeName == goTypeFloat64
}

// isNumericType checks if the type is a numeric type (integer or float).
func isNumericType(typeName string) bool {
	return isIntegerType(typeName) || isFloatType(typeName)
}
