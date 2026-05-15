package world

import (
	"image"
	"testing"
)

func TestNewSurfaceDetailSizing(t *testing.T) {
	sd := NewSurfaceDetail(8, 12)
	if sd.PxWidth != 8*PxPerCell {
		t.Fatalf("PxWidth = %d, want %d", sd.PxWidth, 8*PxPerCell)
	}
	if sd.PxHeight != 12*PxPerCell {
		t.Fatalf("PxHeight = %d, want %d", sd.PxHeight, 12*PxPerCell)
	}
	if got, want := len(sd.Pixels), 8*PxPerCell*12*PxPerCell*4; got != want {
		t.Fatalf("len(Pixels) = %d, want %d", got, want)
	}
	if sd.Dirty {
		t.Fatalf("fresh buffer should not be dirty")
	}
}

func TestStampMaxChannelDiskPeakAndDirtyBox(t *testing.T) {
	sd := NewSurfaceDetail(8, 8)

	// Stamp a 2 px radius disk centred at (10, 10) in pixel space, into G.
	sd.stampMaxChannelDisk(10, 10, 2.0, chTreeWell, 255)

	if !sd.Dirty {
		t.Fatalf("expected Dirty after stamp")
	}
	want := image.Rect(8, 8, 12, 12)
	if sd.DirtyBox != want {
		t.Fatalf("DirtyBox = %v, want %v", sd.DirtyBox, want)
	}

	// Pixel under the centre samples at (9.5, 9.5) → dist = 0.707; falloff
	// = 1 - 0.5/4 = 0.875, so G ≈ 223 (the centre is *between* two pixels).
	centerOff := (10*sd.PxWidth + 10) * 4
	if g := sd.Pixels[centerOff+chTreeWell]; g < 200 {
		t.Fatalf("near-centre G = %d, want >= 200", g)
	}

	// Pixel well outside the radius stays zero.
	outsideOff := (3*sd.PxWidth + 3) * 4
	if g := sd.Pixels[outsideOff+chTreeWell]; g != 0 {
		t.Fatalf("outside G = %d, want 0", g)
	}
}

func TestStampMaxChannelDiskMaxesWithExisting(t *testing.T) {
	sd := NewSurfaceDetail(4, 4)
	// First stamp: peak 100 at (5, 5).
	sd.stampMaxChannelDisk(5, 5, 3.0, chTreeWell, 100)
	first := sd.Pixels[(5*sd.PxWidth+5)*4+chTreeWell]

	// Second stamp at same centre, higher peak — should win on max.
	sd.stampMaxChannelDisk(5, 5, 3.0, chTreeWell, 200)
	second := sd.Pixels[(5*sd.PxWidth+5)*4+chTreeWell]

	if second <= first {
		t.Fatalf("second stamp should max with first: first=%d second=%d", first, second)
	}

	// Third stamp at same centre, lower peak — first should still dominate.
	sd.stampMaxChannelDisk(5, 5, 3.0, chTreeWell, 50)
	third := sd.Pixels[(5*sd.PxWidth+5)*4+chTreeWell]
	if third != second {
		t.Fatalf("lower-peak stamp must not overwrite: got %d, want %d", third, second)
	}
}

func TestZeroChannelClearsButPreservesOthers(t *testing.T) {
	sd := NewSurfaceDetail(4, 4)
	sd.stampMaxChannelDisk(5, 5, 3.0, chTreeWell, 200)
	sd.stampMaxChannelDisk(5, 5, 3.0, chTrack, 200)

	sd.zeroChannel(chTreeWell)
	for i := chTreeWell; i < len(sd.Pixels); i += 4 {
		if sd.Pixels[i] != 0 {
			t.Fatalf("zeroChannel left non-zero G at byte %d: %d", i, sd.Pixels[i])
		}
	}
	// R must survive.
	if sd.Pixels[(5*sd.PxWidth+5)*4+chTrack] == 0 {
		t.Fatalf("zeroChannel(G) must not clear R")
	}
	if !sd.Dirty {
		t.Fatalf("zeroChannel must mark dirty")
	}
}

func TestRestampTreeWellsFillsGNearTrees(t *testing.T) {
	terrain := NewTerrain(20, 20)
	// One dense cell well inside the visible (W-1)×(H-1) tree region.
	terrain.Cells[10][10].TreeDensity = 1.0

	terrain.RestampTreeWells()

	sd := terrain.Surface
	if sd == nil {
		t.Fatalf("Surface must be allocated by NewTerrain")
	}
	// Sample G at the cell centre in pixel space. World metres → pixels
	// via PxPerMeter so the test stays correct regardless of the
	// resolution constant.
	const cellM = 5.0
	worldX := (float32(10) + 0.5) * cellM
	worldZ := (float32(10) + 0.5) * cellM
	cx := int(worldX*PxPerMeter() + 0.5)
	cz := int(worldZ*PxPerMeter() + 0.5)
	off := (cz*sd.PxWidth + cx) * 4
	if sd.Pixels[off+chTreeWell] == 0 {
		t.Fatalf("expected non-zero G under a dense tree cell")
	}
	if !sd.Dirty {
		t.Fatalf("RestampTreeWells must mark dirty")
	}
}
