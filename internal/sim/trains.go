package sim

import (
	"math"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// Trains (notes/next/Trains and Tunnels.md): each railway has one train
// on a timetable. Every Interval clock minutes while the resort's open
// (from two hours before the lifts to half an hour after), it sets off
// from the map's edge, runs in at its style's speed, brakes to the
// platform, stands four clock minutes letting off the guests booked on
// it (as many as it carries; the rest take the next), and runs back out.
// Guests who came by train walk back to the station to leave
// (world.Building.IsDepartPoint).

const (
	trainDwellMinutes = 4.0
	trainBrake        = float32(0.6) // m/s² braking for the platform
	trainAccel        = float32(0.5) // m/s² pulling away
	// railShares is the share of each poll's arriving groups a line of
	// each style takes off the roads; together at most railShareMax.
	railShareMax = float32(0.8)
)

var railShares = [world.RailStyles]float32{0.12, 0.30, 0.45}

// trainServiceHours is whether trains run at clock hour h.
func (s *Simulation) trainsRunning() bool {
	w := s.World
	h := float32(HourOfDay(s.SimTime))
	return w.ResortOpen && h >= w.OpenHour-2 && h < w.CloseHour+0.5
}

// tickTrains runs every line's train.
func (s *Simulation) tickTrains(dt float64) {
	for _, l := range s.World.RailLines {
		if len(l.Track) < 2 {
			continue
		}
		s.tickTrain(l, float32(dt))
	}
}

func (s *Simulation) tickTrain(l *world.RailLine, dt float32) {
	w := s.World
	tr := &l.Train
	spec := l.Style.Spec()
	end := l.Length() - 5 // the front stops just short of the platform's end
	switch tr.State {
	case world.TrainAway:
		if !s.trainsRunning() {
			tr.NextAt = 0
			s.unbook(l) // nobody's coming tonight
			return
		}
		if tr.NextAt == 0 {
			tr.NextAt = s.SimTime // the first of the day sets off now
		}
		if s.SimTime < tr.NextAt {
			return
		}
		tr.State, tr.Front, tr.Speed = world.TrainInbound, 0, spec.Speed
		tr.NextAt = s.SimTime + float64(l.Interval)/60*simSecondsPerHour
		w.AddTrainCost(spec.TripCost)
	case world.TrainInbound:
		tr.Speed = min(spec.Speed, float32(math.Sqrt(float64(2*trainBrake*max(end-tr.Front, 0))))+0.5)
		tr.Front = min(tr.Front+tr.Speed*dt, end)
		if tr.Front >= end {
			tr.State, tr.Speed = world.TrainDwelling, 0
			tr.DwellUntil = s.SimTime + trainDwellMinutes/60*simSecondsPerHour
			s.trainArrives(l)
		}
	case world.TrainDwelling:
		if s.SimTime >= tr.DwellUntil {
			tr.State = world.TrainOutbound
		}
	case world.TrainOutbound:
		tr.Speed = min(spec.Speed, tr.Speed+trainAccel*dt)
		tr.Front -= tr.Speed * dt
		if tr.Front < -spec.TrainLength() {
			tr.State = world.TrainAway
		}
	}
}

// trainArrives lets off the guests booked on line l's train, up to its
// capacity, whole groups at a time, onto the platform.
func (s *Simulation) trainArrives(l *world.RailLine) {
	w := s.World
	st := findBuildingByID(w, l.StationID)
	if st == nil {
		return
	}
	left := l.Style.Spec().Capacity
	spot := l.PlatformSpot()
	_, _, heading := l.Station()
	along := mgl32.Vec2{float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))}
	n := 0
	for len(l.Bound) > 0 && len(l.Bound[0]) <= left {
		grp := l.Bound[0]
		l.Bound = l.Bound[1:]
		left -= len(grp)
		for _, g := range grp {
			// Spread along the platform, as they step off the coaches.
			off := (rng.Global().Float32() - 0.5) * l.Style.Spec().TrainLength() * 0.8
			p := spot.Add(along.Mul(off))
			g.CarLot, g.CarID = st.ID, 0
			if !s.spawnGuestAt(st, g, p, 0) {
				g.CarLot = 0
				g.State = world.AtHome
			}
			n++
		}
	}
}

// unbook sends home everyone booked on line l's next train.
func (s *Simulation) unbook(l *world.RailLine) {
	for _, grp := range l.Bound {
		for _, g := range grp {
			g.State = world.AtHome
		}
	}
	l.Bound = l.Bound[:0]
}

// bookByRail takes a share of the arriving groups off the roads onto the
// trains (all of them if there's nowhere to park): booked on a line's
// next train, picked by its style's pull. Returns the groups left to come
// by car.
func (s *Simulation) bookByRail(groups [][]*world.Guest, parking bool) [][]*world.Guest {
	w := s.World
	if len(w.RailLines) == 0 {
		return groups
	}
	var total float32
	for _, l := range w.RailLines {
		total += railShares[l.Style]
	}
	share := min(total, railShareMax)
	if !parking {
		share = 1
	}
	kept := groups[:0]
	for _, grp := range groups {
		if rng.Global().Float32() >= share {
			kept = append(kept, grp)
			continue
		}
		pick := rng.Global().Float32() * total
		var line *world.RailLine
		for _, l := range w.RailLines {
			if pick -= railShares[l.Style]; pick <= 0 || line == nil {
				line = l
				if pick <= 0 {
					break
				}
			}
		}
		for _, g := range grp {
			g.State = world.InCar // on the way, by train
		}
		line.Bound = append(line.Bound, grp)
	}
	return kept
}
