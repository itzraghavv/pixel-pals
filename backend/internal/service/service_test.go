package service

import "testing"

func TestDimensions(t *testing.T) {
	for _, s := range []string{"0x10", "-1x10", "10x0", "10X10", "10x10x10", "1.5x2", "999999999999x2", "1001x2", ""} {
		if _, _, e := Dimensions(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if w, h, e := Dimensions("20x30"); e != nil || w != 20 || h != 30 {
		t.Fatal(w, h, e)
	}
}
func TestFootprintBounds(t *testing.T) {
	for _, v := range []struct {
		x, y, ew, eh int32
		ok           bool
	}{{0, 0, 1, 1, true}, {9, 9, 1, 1, true}, {10, 0, 1, 1, false}, {9, 9, 2, 1, false}, {8, 8, 2, 2, true}, {-1, 0, 1, 1, false}, {0, 0, 11, 1, false}} {
		if Fits(v.x, v.y, 10, 10, v.ew, v.eh) != v.ok {
			t.Errorf("%+v", v)
		}
	}
}
