package main

import "testing"

func TestToMultiPolygon(t *testing.T) {
	in := []string{"-29.7839 152.3404 -29.7839 152.3405 -29.7839 152.3404", "-29.7848 152.3375 -29.7848 152.3375 -29.7847 152.3375 -29.7848 152.3375"}
	out := "MULTIPOLYGON(((152.3404 -29.7839, 152.3405 -29.7839, 152.3404 -29.7839)),((152.3375 -29.7848, 152.3375 -29.7848, 152.3375 -29.7847, 152.3375 -29.7848)))"
	if g := toMultiPolygon(in); g != out {
		t.Errorf("toMultiPolygon(%v) = %v, want %v", in, g, out)
	}
}
