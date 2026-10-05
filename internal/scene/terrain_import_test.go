package scene

import "testing"

func TestClipSegment(t *testing.T) {
	near := func(a, b float32) bool { return a-b < 1e-3 && b-a < 1e-3 }
	x0, y0, x1, y1, ok := clipSegment(-10, 50, 110, 50, 0, 32, 100, 200)
	if !ok || !near(x0, 0) || !near(x1, 100) || !near(y0, 50) || !near(y1, 50) {
		t.Errorf("horizontal = %v %v %v %v %v", x0, y0, x1, y1, ok)
	}
	if _, _, _, _, ok := clipSegment(10, 0, 90, 20, 0, 32, 100, 200); ok {
		t.Error("segment above the map area should be dropped")
	}
	if x0, y0, _, _, ok := clipSegment(50, 0, 50, 64, 0, 32, 100, 200); !ok || !near(x0, 50) || !near(y0, 32) {
		t.Errorf("vertical = %v %v %v", x0, y0, ok)
	}
}
