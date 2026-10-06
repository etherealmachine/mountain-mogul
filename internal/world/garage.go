package world

import "github.com/go-gl/mathgl/mgl32"

// A snowcat garage (ServiceGarage tiles in a building) holds the vehicles
// the resort buys: snowcats for grooming and snowmobiles for patrol. What
// fits comes from its floor space, the way a service's capacity comes from
// its tiles: a snowcat needs a 10 × 10 m square of garage, and a garage
// tile holds two snowmobiles. Only the ground floor counts, so a lodge's
// upper storeys add no vehicle space. Space is counted in half-tiles.

const (
	// CatGarageHalfTiles is a snowcat's garage space: 4 tiles.
	CatGarageHalfTiles = 8
	// SnowmobileGarageHalfTiles is a snowmobile's: half a tile.
	SnowmobileGarageHalfTiles = 1

	// SnowmobilePrice is what buying a snowmobile costs; selling a
	// vehicle refunds half its price.
	SnowmobilePrice = 15_000
)

// Snowmobile is one patrol snowmobile, housed in a garage (GarageID).
// Overnight it's parked inside; in the day a patroller takes it out
// (notes/next/Patrol Day.md).
type Snowmobile struct {
	ID       uint64
	GarageID uint64
	Pos      mgl32.Vec3
	Heading  float32
	InGarage bool   // parked inside its garage (not drawn)
	TakenBy  uint64 // the patroller who has it out today; 0 when free
}

// GarageSpace is b's vehicle space and what's used, in half-tiles.
func (w *World) GarageSpace(b *Building) (used, total int) {
	total = 2 * b.TileCount(ServiceGarage)
	used = len(w.CatsOwnedBy(b.ID)) * CatGarageHalfTiles
	for _, m := range w.Snowmobiles {
		if m.GarageID == b.ID {
			used += SnowmobileGarageHalfTiles
		}
	}
	return used, total
}

// SnowmobilesIn is the snowmobiles housed in garage b.
func (w *World) SnowmobilesIn(b *Building) []*Snowmobile {
	var out []*Snowmobile
	for _, m := range w.Snowmobiles {
		if m.GarageID == b.ID {
			out = append(out, m)
		}
	}
	return out
}

// GarageFits reports whether b has room for a vehicle of half halfTiles.
func (w *World) GarageFits(b *Building, half int) bool {
	used, total := w.GarageSpace(b)
	return used+half <= total
}

// GarageCanLose reports whether b's garage can give up n tiles without
// leaving its vehicles nowhere to park.
func (w *World) GarageCanLose(b *Building, n int) bool {
	used, total := w.GarageSpace(b)
	return used <= total-2*n
}

// AddSnowmobile houses a new snowmobile in garage b, parked inside.
// Cost gating and the space check live in the caller.
func (w *World) AddSnowmobile(b *Building) *Snowmobile {
	m := &Snowmobile{ID: w.NextID(), GarageID: b.ID, Pos: w.GarageSpot(b), InGarage: true}
	w.Snowmobiles = append(w.Snowmobiles, m)
	return m
}

// RemoveSnowmobile drops one snowmobile; a patroller who had it out
// carries on without one.
func (w *World) RemoveSnowmobile(id uint64) {
	for _, p := range w.Patrollers {
		if p.SnowmobileID == id {
			p.SnowmobileID = 0
		}
	}
	for i, m := range w.Snowmobiles {
		if m.ID == id {
			w.Snowmobiles = append(w.Snowmobiles[:i], w.Snowmobiles[i+1:]...)
			return
		}
	}
}

// SnowmobileByID returns the snowmobile with id, or nil.
func (w *World) SnowmobileByID(id uint64) *Snowmobile {
	if id == 0 {
		return nil
	}
	for _, m := range w.Snowmobiles {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// RemoveSnowmobilesIn drops every snowmobile housed in garage b.
func (w *World) RemoveSnowmobilesIn(id uint64) {
	var gone []uint64
	for _, m := range w.Snowmobiles {
		if m.GarageID == id {
			gone = append(gone, m.ID)
		}
	}
	for _, m := range gone {
		w.RemoveSnowmobile(m)
	}
}

// GarageSpot is a point inside garage b: the centre of its first garage
// tile, on the floor.
func (w *World) GarageSpot(b *Building) mgl32.Vec3 {
	for _, c := range b.Cells {
		if b.ServiceAt(c) == ServiceGarage {
			p := b.TileCentre(c)
			return mgl32.Vec3{p[0], w.ShellFloorY(b), p[1]}
		}
	}
	return w.ServiceHome(b, ServiceGarage)
}

// TrimGarage drops, without a refund, vehicles that no longer fit b's
// garage (a hand-edited save, or tiles lost some other way): snowmobiles
// first, then cats.
func (w *World) TrimGarage(b *Building) {
	for used, total := w.GarageSpace(b); used > total; used, total = w.GarageSpace(b) {
		if ms := w.SnowmobilesIn(b); len(ms) > 0 {
			w.RemoveSnowmobile(ms[len(ms)-1].ID)
			continue
		}
		cats := w.CatsOwnedBy(b.ID)
		if len(cats) == 0 {
			return
		}
		w.RemoveSnowcat(cats[len(cats)-1].ID)
	}
}

// SettleSnowmobiles keeps snowmobiles and patrollers agreeing on who has
// which out: a snowmobile whose patroller is gone (or doesn't have it)
// goes back in its garage, and a patroller whose snowmobile is gone is
// left without one. Called after loading.
func (w *World) SettleSnowmobiles() {
	has := map[uint64]uint64{} // snowmobile → patroller
	for _, p := range w.Patrollers {
		if p.SnowmobileID != 0 {
			if w.SnowmobileByID(p.SnowmobileID) == nil {
				p.SnowmobileID = 0
				continue
			}
			has[p.SnowmobileID] = p.ID
		}
	}
	for _, m := range w.Snowmobiles {
		if m.TakenBy != 0 && has[m.ID] != m.TakenBy {
			w.returnSnowmobile(m)
		}
	}
}

// returnSnowmobile puts a snowmobile back inside its garage, free.
func (w *World) returnSnowmobile(m *Snowmobile) {
	m.TakenBy, m.InGarage = 0, true
	if g := w.BuildingByID(m.GarageID); g != nil {
		m.Pos = w.GarageSpot(g)
	}
}
