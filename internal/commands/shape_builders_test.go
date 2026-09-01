package commands

import "github.com/gaborage/go-bricks-openapi/internal/models"

// Test-only TypeShape builders mirroring what the analyzer's decoder stamps.
func prim(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePrimitive, Name: name}
}
func named(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}
func sliceOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeSlice, Elem: &s}
}
