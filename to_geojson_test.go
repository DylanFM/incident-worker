package main

import "testing"

func TestSqrt(t *testing.T) {
	const in = "-33.6097 150.0216"
	const out = `{"type":"Point","coordinates":[150.0216,-33.6097]}`
	if g, _ := toGeoJsonPoints(in); g != out {
		t.Errorf("toGeoJsonPoints(%v) = %v, want %v", in, g, out)
	}
}
