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
func arrayOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeArray, Elem: &s}
}
func mapOf(k, v models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeMap, Key: &k, Elem: &v}
}
func unknownShape() models.TypeShape { return models.TypeShape{Kind: models.ShapeUnknown} }

// payloadSlice is the *TypeShape a response TypeInfo carries for a []T payload.
func payloadSlice(s models.TypeShape) *models.TypeShape {
	sh := sliceOf(s)
	return &sh
}

// payloadArray is the *TypeShape a response TypeInfo carries for a [N]T payload.
func payloadArray(s models.TypeShape) *models.TypeShape {
	sh := arrayOf(s)
	return &sh
}

// Resolution-only leaves, as the analyzer's resolver stamps them.
func refTo(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeRef, Name: name}
}
func kindOnlyLeaf(kind string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeKindOnly, Name: kind}
}
func marshalerOf(name string, under models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeMarshaler, Name: name, Elem: &under}
}
func recursiveLeaf(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeRecursive, Name: name}
}

// withResolution returns a copy of f whose Resolution is r.
func withResolution(f *models.FieldInfo, r models.TypeShape) *models.FieldInfo {
	c := *f
	c.Resolution = &r
	return &c
}

// substNamed returns s with every ShapeNamed leaf replaced by leaf: the
// Resolution the analyzer stamps when each named type resolves to leaf.
func substNamed(s, leaf models.TypeShape) models.TypeShape {
	switch s.Kind {
	case models.ShapeNamed:
		return leaf
	case models.ShapePointer, models.ShapeSlice, models.ShapeArray, models.ShapeMap:
		if s.Elem != nil {
			e := substNamed(*s.Elem, leaf)
			s.Elem = &e
		}
		return s
	default:
		return s
	}
}

// resolvedTo returns a copy of f whose Resolution is its Shape with every
// named leaf resolved to leaf.
func resolvedTo(f *models.FieldInfo, leaf models.TypeShape) *models.FieldInfo {
	return withResolution(f, substNamed(f.Shape, leaf))
}

// resolvedPayload returns ti with its Resolution set to a copy of its Shape,
// as the analyzer stamps a well-known or builtin payload, whose leaves resolve
// to themselves.
func resolvedPayload(ti *models.TypeInfo) *models.TypeInfo {
	r := *ti.Shape
	ti.Resolution = &r
	return ti
}
