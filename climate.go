// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"math"

	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2ter"
)

// HexKm is a campaign hex's flat-to-flat height in campaign kilometers.
const HexKm = 10

// Climate is a land hex's climate. Values are rounded as written to JSON,
// and the biome and surface are chosen from the rounded values, so they can
// be checked from the output alone.
type Climate struct {
	// Latitude of the hex's center, in degrees, north positive.
	Latitude float64 `json:"latitude_deg"`
	// Temperature is the mean annual temperature, and Warmest and Coldest
	// the means of the warmest and coldest months, in °C.
	Temperature float64 `json:"temperature_c"`
	Warmest     float64 `json:"warmest_c"`
	Coldest     float64 `json:"coldest_c"`
	// Precipitation is the annual precipitation in mm.
	Precipitation float64 `json:"precipitation_mm"`
	// Moisture is the share of the sea air's moisture left when the
	// prevailing wind reaches the hex, and Lift the height in meters the
	// wind climbs onto the hex (see Rules).
	Moisture float64 `json:"moisture"`
	Lift     float64 `json:"lift_m"`
}

// Map looks up hexes by column and row.
type Map struct {
	Grid  hmz2ele.Grid
	Hexes []hmz2ter.HexJSON
	slot  []int32
}

// NewMap indexes the hexes of the grid.
func NewMap(g hmz2ele.Grid, hexes []hmz2ter.HexJSON) *Map {
	m := &Map{Grid: g, Hexes: hexes, slot: make([]int32, g.Columns*g.Rows)}
	for i := range m.slot {
		m.slot[i] = -1
	}
	for i, h := range hexes {
		m.slot[h.Row*g.Columns+h.Col] = int32(i)
	}
	return m
}

// At returns the hex of the pixel containing the raster point, or nil if
// the point is outside the raster or its hex isn't in the grid. A pixel
// belongs to the hex containing its center, as in hmz2ter; going through
// the pixel keeps points that fall exactly on a hex boundary (the trace
// often steps along one) from depending on floating-point noise.
func (m *Map) At(x, y float64) *hmz2ter.HexJSON {
	if x < 0 || y < 0 || x >= float64(m.Grid.Width) || y >= float64(m.Grid.Height) {
		return nil
	}
	// Snap to 6 decimals first: with the 60° trades the exact y of every
	// step is an integer, on the line between two pixel rows, and the
	// floor must not depend on rounding error.
	px, py := math.Floor(math.Round(x*1e6)/1e6), math.Floor(math.Round(y*1e6)/1e6)
	col, row := m.Grid.HexAt(px+0.5, py+0.5)
	if col < 0 || row < 0 || col >= m.Grid.Columns || row >= m.Grid.Rows {
		return nil
	}
	if i := m.slot[row*m.Grid.Columns+col]; i >= 0 {
		return &m.Hexes[i]
	}
	return nil
}

// isSea reports whether the wind picks up moisture over the hex. Border cuts
// count as sea, because the game world has sea beyond them, and so does
// anything off the map.
func isSea(h *hmz2ter.HexJSON) bool {
	if h == nil {
		return true
	}
	switch h.Landform {
	case hmz2ter.SaltWater, hmz2ter.Cliffs, hmz2ter.Badlands:
		return true
	}
	return false
}

// elevation is the height the wind crosses a hex at: its median elevation,
// or 0 m for sea.
func elevation(h *hmz2ter.HexJSON) float64 {
	if isSea(h) || h.Elevation == nil {
		return 0
	}
	return float64(h.Elevation.Median)
}

// Profile returns the elevations the wind crosses on its way to the hex,
// from a bearing (clockwise from north): 0 m for the sea it starts from, the
// elevation every StepKm upwind, farthest first, and the hex's own. The
// trace stops at the first sea, and after ReachKm.
func (m *Map) Profile(h *hmz2ter.HexJSON, from float64, r Rules) []float64 {
	x, y := m.Grid.Center(h.Col, h.Row)
	pxPerKm := 2 * float64(m.Grid.Apothem) / HexKm
	rad := from * math.Pi / 180
	dx, dy := math.Sin(rad)*r.StepKm*pxPerKm, -math.Cos(rad)*r.StepKm*pxPerKm
	var upwind []float64
	for i := 1; float64(i)*r.StepKm <= r.ReachKm; i++ {
		u := m.At(x+float64(i)*dx, y+float64(i)*dy)
		if isSea(u) {
			break
		}
		upwind = append(upwind, elevation(u))
	}
	profile := make([]float64, 0, len(upwind)+2)
	profile = append(profile, 0)
	for i := len(upwind) - 1; i >= 0; i-- {
		profile = append(profile, upwind[i])
	}
	return append(profile, elevation(h))
}

// Moisture returns the moisture left at the end of the profile, and the lift
// onto its last point.
func (r Rules) Moisture(profile []float64) (moisture, lift float64) {
	moisture = 1
	for i := 1; i < len(profile); i++ {
		rise := max(0, profile[i]-profile[i-1])
		moisture *= math.Exp(-r.StepKm/r.RainoutKm - rise/r.OrographicM)
	}
	n := len(profile) - 1
	window := int(math.Round(r.LiftWindowKm / r.StepKm))
	low := profile[n]
	for i := max(0, n-window); i < n; i++ {
		low = min(low, profile[i])
	}
	return moisture, profile[n] - low
}

// Climate returns a land hex's climate. Everything uses the latitude rounded
// as written to JSON.
func (m *Map) Climate(h *hmz2ter.HexJSON, r Rules) Climate {
	_, y := m.Grid.Center(h.Col, h.Row)
	lat := round(r.Latitude(y, m.Grid.Height), 2)
	t := r.SeaLevelTemp.At(lat) - r.LapseRate*elevation(h)/1000
	span := r.RangeBase + r.RangePerDeg*math.Abs(lat)

	trades, westerlies, w := r.Wind(lat)
	var moisture, lift float64
	if w < 1 {
		mo, li := r.Moisture(m.Profile(h, trades, r))
		moisture, lift = (1-w)*mo, (1-w)*li
	}
	if w > 0 {
		mo, li := r.Moisture(m.Profile(h, westerlies, r))
		moisture, lift = moisture+w*mo, lift+w*li
	}
	c := Climate{
		Latitude:    lat,
		Temperature: round(t, 1),
		Warmest:     round(t+span/2, 1),
		Coldest:     round(t-span/2, 1),
		Moisture:    round(moisture, 3),
		Lift:        round(lift, 0),
	}
	share := r.ConvectiveShare.At(lat)
	p := r.WindwardPrecip.At(lat) * (share + (1-share)*c.Moisture*(1+r.LiftGain*c.Lift/1000))
	c.Precipitation = round(p, 0)
	return c
}

// round rounds half away from zero to the given number of decimals, after
// first rounding to 6 decimals, so that floating-point noise can't turn an
// exact half (such as 22.25) into 22.2499999.
func round(v float64, decimals int) float64 {
	v = math.Round(v*1e6) / 1e6
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
