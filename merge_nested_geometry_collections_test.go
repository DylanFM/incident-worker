package main

import (
	"github.com/paulmach/go.geojson"
	"reflect"
	"testing"
)

func TestMergeNested(t *testing.T) {
	nested := geojson.NewCollectionGeometry(
		geojson.NewPointGeometry([]float64{1, 2}),
		geojson.NewMultiPointGeometry([]float64{1, 2}, []float64{3, 4}),
		geojson.NewCollectionGeometry(
			geojson.NewPointGeometry([]float64{1, 2}),
			geojson.NewCollectionGeometry(
				geojson.NewMultiPointGeometry([]float64{1, 2}, []float64{3, 4}),
			),
		),
	)

	merged := mergeNestedGeometryCollections(nested)

	if !merged.IsCollection() {
		t.Error("Is no longer a collection")
	}

	num := len(merged.Geometries)
	if num != 4 {
		t.Errorf("Expected it to now have 4 geometries, has %v", num)
	}
}

func TestMergeIgnore(t *testing.T) {
	point := geojson.NewPointGeometry([]float64{1, 2})

	merged := mergeNestedGeometryCollections(point)

	j1, _ := point.MarshalJSON()
	j2, _ := merged.MarshalJSON()

	if !reflect.DeepEqual(j1, j2) {
		t.Error("Marshaled JSON differs, seems merge function changed a point")
	}
}
