package main

import "testing"

func TestToPoint(t *testing.T) {
	const in = "-33.6097 150.0216"
	const out = `POINT(150.0216 -33.6097)`
	if g := toPoint(in); g != out {
		t.Errorf("toPoint(%v) = %v, want %v", in, g, out)
	}
}
