package geo

import "math"

// LatLonToUTM projects a point onto UTM zone (1–60, northern hemisphere)
// on the GRS80 ellipsoid (NAD83; within a metre of WGS84), returning
// easting and northing in metres. Transverse Mercator series from
// Snyder, "Map Projections — A Working Manual", eqs. 8-9 to 8-10;
// sub-millimetre within a zone.
func LatLonToUTM(lat, lon float64, zone int) (e, n float64) {
	const (
		a  = 6378137.0
		f  = 1 / 298.257222101
		k0 = 0.9996
	)
	e2 := f * (2 - f)
	ep2 := e2 / (1 - e2)
	phi := lat * math.Pi / 180
	lam0 := float64(zone*6-183) * math.Pi / 180
	lam := lon * math.Pi / 180

	sin, cos := math.Sin(phi), math.Cos(phi)
	tan := sin / cos
	N := a / math.Sqrt(1-e2*sin*sin)
	T := tan * tan
	C := ep2 * cos * cos
	A := cos * (lam - lam0)
	M := a * ((1-e2/4-3*e2*e2/64-5*e2*e2*e2/256)*phi -
		(3*e2/8+3*e2*e2/32+45*e2*e2*e2/1024)*math.Sin(2*phi) +
		(15*e2*e2/256+45*e2*e2*e2/1024)*math.Sin(4*phi) -
		(35*e2*e2*e2/3072)*math.Sin(6*phi))

	e = k0*N*(A+(1-T+C)*A*A*A/6+(5-18*T+T*T+72*C-58*ep2)*A*A*A*A*A/120) + 500000
	n = k0 * (M + N*tan*(A*A/2+(5-T+9*C+4*C*C)*A*A*A*A/24+(61-58*T+T*T+600*C-330*ep2)*A*A*A*A*A*A/720))
	return e, n
}

// UTMZone is the standard UTM zone for a longitude.
func UTMZone(lon float64) int {
	return int(math.Floor((lon+180)/6)) + 1
}
