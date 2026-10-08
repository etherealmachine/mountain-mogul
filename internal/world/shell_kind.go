package world

// ShellKind is what a service building is built as. They differ in cost,
// upkeep, look, and height; a lodge alone can rise to three storeys.
// What heating means for guests, and how much nicer a lodge is than a
// tent, come later (notes/next/Rotated Buildings.md).
type ShellKind uint8

const (
	ShellLodge ShellKind = iota // timber and stone, heated, up to three storeys
	ShellTent                   // fabric on an aluminium frame, heated, one storey
	ShellShed                   // corrugated metal, unheated, one storey
	ShellKindCount
)

// Label is the kind's player-facing name.
func (k ShellKind) Label() string {
	switch k {
	case ShellTent:
		return "Tent"
	case ShellShed:
		return "Shed"
	}
	return "Lodge"
}

// Heated reports whether the kind is heated.
func (k ShellKind) Heated() bool { return k != ShellShed }

// MaxStoreys is how many storeys the kind can have.
func (k ShellKind) MaxStoreys() int {
	if k == ShellLodge {
		return 3
	}
	return 1
}

// WallHeight is one storey's wall height. Keep in sync with
// models-src/lib/lodge_kit.scad.
func (k ShellKind) WallHeight() float32 {
	switch k {
	case ShellTent:
		return 3.0
	case ShellShed:
		return 4.0
	}
	return 5.0
}

// RoofRise is the roof's rise per tile of run. Keep in sync with
// models-src/lib/lodge_kit.scad.
func (k ShellKind) RoofRise() float32 {
	switch k {
	case ShellTent:
		return 2.6 // a steep peaked fabric roof
	case ShellShed:
		return 0.8 // a low metal roof
	}
	return 2.1 // ~40°
}

// BaseCost is charged with a building's first tile: foundations and
// utilities.
func (k ShellKind) BaseCost() int {
	switch k {
	case ShellTent:
		return 15_000
	case ShellShed:
		return 10_000
	}
	return ServiceBuildingBaseCost
}

// tileCostScale scales a service's tile cost for the kind.
func (k ShellKind) tileCostScale() float32 {
	switch k {
	case ShellTent:
		return 0.4
	case ShellShed:
		return 0.25
	}
	return 1
}

// DailyBaseCost is one open day's base upkeep for a building of the kind.
func (k ShellKind) DailyBaseCost() int {
	switch k {
	case ShellTent:
		return 120 // heating a tent isn't cheap
	case ShellShed:
		return 60
	}
	return ServiceBuildingDailyBaseCost
}

// dailyScale scales a service's per-tile upkeep for the kind.
func (k ShellKind) dailyScale() float32 {
	switch k {
	case ShellTent:
		return 0.8
	case ShellShed:
		return 0.5
	}
	return 1
}

// TileCost is what one tile of s costs in a building of kind k, for one
// storey.
func (k ShellKind) TileCost(s Service) int {
	return int(float32(s.TileCost()) * k.tileCostScale())
}

// structureTileCost is the walls, roof, and floor of one lodge tile; a
// service's tile cost covers it and its fit-out together.
const structureTileCost = 10_000

// StructureTileCost is one tile of empty floor in kind k, per storey.
func (k ShellKind) StructureTileCost() int {
	return int(structureTileCost * k.tileCostScale())
}

// FitOutTileCost is fitting service s into one tile of kind k, per
// storey: the service's tile cost less the structure already paid for.
func (k ShellKind) FitOutTileCost(s Service) int {
	if s == ServiceNone {
		return 0
	}
	return max(k.TileCost(s)-k.StructureTileCost(), 0)
}

// Floors is b's storey count, at least 1.
func (b *Building) Floors() int {
	return max(b.Storeys, 1)
}

// AddStoreyCost is what adding a storey to b costs: every tile again,
// its structure and its fit-out.
func (b *Building) AddStoreyCost() int {
	cost := 0
	for _, c := range b.Cells {
		cost += b.Kind.StructureTileCost() + b.Kind.FitOutTileCost(b.ServiceAt(c))
	}
	return cost
}

// ShellMeshIndex is the kit tile's index across every kind's kit: kind k's
// tiles follow kind k-1's.
func ShellMeshIndex(k ShellKind, t ShellTileKind) uint32 {
	return uint32(k)*uint32(ShellTileKindCount) + uint32(t)
}
