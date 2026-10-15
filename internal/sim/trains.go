package sim

import (
	"math"

	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"
)

// Trains (notes/next/Rail Network.md): every Interval clock minutes while
// the resort's open (from two hours before the lifts to half an hour
// after), a train sets off for each station that has a route
// (world.StationRoute): in from an entry, stopping at every station it
// passes, letting off the guests booked for each (as many as it has seats
// for; the rest take the next), and out again, through to another entry
// or back the way it came. It runs at each stretch's speed and brakes
// for its stops. Guests who came by train walk back to their station to
// leave (world.Building.IsDepartPoint).

const (
	trainDwellMinutes = 4.0
	trainBrake        = float32(0.6) // m/s² braking for a stop
	trainAccel        = float32(0.5) // m/s² pulling away
	// railShares is the share of each poll's arriving groups a station
	// served by each style of train takes off the roads; together at
	// most railShareMax.
	railShareMax = float32(0.8)
)

var railShares = [world.RailStyles]float32{0.12, 0.30, 0.45}

// trainsRunning is whether trains run now.
func (s *Simulation) trainsRunning() bool {
	w := s.World
	h := float32(HourOfDay(s.SimTime))
	return w.ResortOpen && h >= w.OpenHour-2 && h < w.CloseHour+0.5
}

// tickTrains sets off each station's trains when they're due, and runs
// the trains on their way.
func (s *Simulation) tickTrains(dt float64) {
	w := s.World
	running := s.trainsRunning()
	for _, st := range w.RailStations {
		if !running {
			st.NextAt = 0
			s.unbook(st) // nobody's coming tonight
			continue
		}
		if st.NextAt == 0 {
			st.NextAt = s.SimTime // the first of the day sets off now
		}
		if s.SimTime < st.NextAt {
			continue
		}
		st.NextAt = s.SimTime + float64(st.Interval)/60*simSecondsPerHour
		route, style, _ := w.StationRoute(st, st.Flip)
		st.Flip = !st.Flip
		if route == nil {
			s.unbook(st)
			continue
		}
		spec := style.Spec()
		w.Trains = append(w.Trains, &world.Train{Route: route, Style: style, Speed: spec.Speed, Seats: spec.Capacity})
		w.AddTrainCost(spec.TripCost)
	}
	kept := w.Trains[:0]
	for _, tr := range w.Trains {
		if s.tickTrain(tr, float32(dt)) {
			kept = append(kept, tr)
		}
	}
	w.Trains = kept
}

// tickTrain runs train tr on dt seconds; false once it's gone beyond the
// map.
func (s *Simulation) tickTrain(tr *world.Train, dt float32) bool {
	return runTrain(tr, dt, s.SimTime, func(st *world.RailStation) { s.trainArrives(tr, st) })
}

// runTrain runs train tr on dt seconds at sim time now, arrive called at
// each stop; false once it's gone beyond the map.
func runTrain(tr *world.Train, dt float32, now float64, arrive func(*world.RailStation)) bool {
	spec := tr.Style.Spec()
	r := tr.Route
	length := spec.TrainLength()
	if tr.Dwelling {
		if now < tr.DwellUntil {
			return true
		}
		tr.Dwelling = false
		tr.Stop++
		if r.Back && tr.Stop == len(r.Stops) && !tr.Reversed {
			// Turned round at the last stop: the back's now the front.
			tr.Reversed = true
			tr.Front = r.Length - (tr.Front - length)
		}
	}
	// The stop ahead: its front stops half a train past the platform's
	// middle.
	stopAt := float32(math.MaxFloat32)
	if !tr.Reversed && tr.Stop < len(r.Stops) {
		stopAt = r.Stops[tr.Stop].At + length/2
		if r.Back {
			stopAt = min(stopAt, r.Length/2) // where it turns
		}
	}
	limit := spec.Speed
	if t := r.TrackAt(tr.Front); t != nil {
		limit = min(limit, t.Style.Spec().Speed)
	}
	brake := float32(math.Sqrt(float64(2*trainBrake*max(stopAt-tr.Front, 0)))) + 0.5
	tr.Speed = min(tr.Speed+trainAccel*dt, limit, brake)
	tr.Front = min(tr.Front+tr.Speed*dt, stopAt)
	if tr.Front >= stopAt {
		tr.Speed, tr.Dwelling = 0, true
		tr.DwellUntil = now + trainDwellMinutes/60*simSecondsPerHour
		arrive(r.Stops[tr.Stop].Station)
	}
	return tr.Front < r.Length+length
}

// trainArrives lets off the guests booked for station st, as many as the
// train has seats for, whole groups at a time, onto the platform.
func (s *Simulation) trainArrives(tr *world.Train, st *world.RailStation) {
	w := s.World
	bldg := findBuildingByID(w, st.ID)
	if bldg == nil {
		return
	}
	spot := w.PlatformSpot(st)
	_, _, heading, _, _ := w.StationFrame(st)
	along := [2]float32{float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))}
	for len(st.Bound) > 0 && len(st.Bound[0]) <= tr.Seats {
		grp := st.Bound[0]
		st.Bound = st.Bound[1:]
		tr.Seats -= len(grp)
		for _, g := range grp {
			// Spread along the platform, as they step off the coaches.
			off := (rng.Global().Float32() - 0.5) * tr.Style.Spec().TrainLength() * 0.8
			p := spot
			p[0] += along[0] * off
			p[1] += along[1] * off
			g.CarLot, g.CarID = st.ID, 0
			if !s.spawnGuestAt(bldg, g, p, 0) {
				g.CarLot = 0
				g.State = world.AtHome
			}
		}
	}
}

// unbook sends home everyone booked to come to station st.
func (s *Simulation) unbook(st *world.RailStation) {
	for _, grp := range st.Bound {
		for _, g := range grp {
			g.State = world.AtHome
		}
	}
	st.Bound = st.Bound[:0]
}

// bookByRail takes a share of the arriving groups off the roads onto the
// trains (all of them if there's nowhere to park): booked on a served
// station's next train, picked by its train's pull. Returns the groups
// left to come by car.
func (s *Simulation) bookByRail(groups [][]*world.Guest, parking bool) [][]*world.Guest {
	served, pulls, total := s.servedStations()
	if len(served) == 0 {
		return groups
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
		st := served[len(served)-1]
		for i, p := range pulls {
			if pick -= p; pick <= 0 {
				st = served[i]
				break
			}
		}
		for _, g := range grp {
			g.State = world.InCar // on the way, by train
		}
		st.Bound = append(st.Bound, grp)
	}
	return kept
}

// servedStations is the stations with trains, each one's pull (its
// train's share), and their total.
func (s *Simulation) servedStations() ([]*world.RailStation, []float32, float32) {
	var served []*world.RailStation
	var pulls []float32
	var total float32
	for _, st := range s.World.RailStations {
		if r, style, _ := s.World.StationRoute(st, false); r != nil {
			served = append(served, st)
			pulls = append(pulls, railShares[style])
			total += railShares[style]
		}
	}
	return served, pulls, total
}

// anyServedStation reports whether any station has trains.
func (s *Simulation) anyServedStation() bool {
	served, _, _ := s.servedStations()
	return len(served) > 0
}
