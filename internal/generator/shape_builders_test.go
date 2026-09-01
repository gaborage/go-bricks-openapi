package generator

import "github.com/gaborage/go-bricks-openapi/internal/models"

// Test-only TypeShape builders mirroring what the analyzer's decoder stamps.
// The generator has no string-parsing fallback, so every hand-built FieldInfo
// must carry a Shape.
func prim(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePrimitive, Name: name}
}
func named(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}
func ptrOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePointer, Elem: &s}
}
func sliceOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeSlice, Elem: &s}
}
func mapOf(k, v models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeMap, Key: &k, Elem: &v}
}
