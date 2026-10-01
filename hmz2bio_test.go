// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2ter"
)

func TestTableAt(t *testing.T) {
	tab := Table{{0, 10}, {10, 20}, {30, 0}}
	for _, tc := range []struct{ lat, want float64 }{
		{0, 10}, {5, 15}, {-5, 15}, {10, 20}, {20, 10}, {30, 0}, {45, 0}, {-45, 0},
	} {
		if got := tab.At(tc.lat); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("At(%g) = %g, want %g", tc.lat, got, tc.want)
		}
	}
}

func TestWind(t *testing.T) {
	r := DefaultRules()
	for _, tc := range []struct{ lat, trades, westerlies, weight float64 }{
		{10, 60, 255, 0},
		{27, 60, 255, 0},
		{30, 60, 255, 0.5},
		{40, 60, 255, 1},
		{-10, 120, -75, 0},
	} {
		tr, we, w := r.Wind(tc.lat)
		if tr != tc.trades || we != tc.westerlies || math.Abs(w-tc.weight) > 1e-9 {
			t.Errorf("Wind(%g) = %g, %g, %g; want %g, %g, %g", tc.lat, tr, we, w, tc.trades, tc.westerlies, tc.weight)
		}
	}
}

func TestMoisture(t *testing.T) {
	r := DefaultRules()
	step := math.Exp(-r.StepKm / r.RainoutKm)
	// A flat coastal hex: one step from the sea, no rise.
	if m, lift := r.Moisture([]float64{0, 0}); math.Abs(m-step) > 1e-12 || lift != 0 {
		t.Errorf("flat coast: %g, %g; want %g, 0", m, lift, step)
	}
	// Climbing 1,500 m costs a factor of e; descending costs nothing.
	up := []float64{0, 1500, 0}
	want := step * step * math.Exp(-1)
	if m, lift := r.Moisture(up); math.Abs(m-want) > 1e-12 || lift != 0 {
		t.Errorf("over a ridge: %g, %g; want %g, 0", m, lift, want)
	}
	// Lift is measured from the lowest point within the window only.
	profile := []float64{0, 100, 100, 100, 100, 100, 100, 100, 400}
	if _, lift := r.Moisture(profile); lift != 300 {
		t.Errorf("lift = %g, want 300 (the sea is outside the 30 km window)", lift)
	}
	if _, lift := r.Moisture([]float64{0, 100, 400}); lift != 400 {
		t.Errorf("lift = %g, want 400 (the sea is inside the window)", lift)
	}
}

// A synthetic map: a column of land hexes with sea to the east. The trades
// come from the east (90°), so every hex's profile runs back to that sea.
func TestProfile(t *testing.T) {
	g, err := hmz2ele.NewGrid(48, 1000, 400)
	if err != nil {
		t.Fatal(err)
	}
	var hexes []hmz2ter.HexJSON
	for row := range g.Rows {
		for col := range g.Columns {
			if !g.Contains(col, row) {
				continue
			}
			h := hmz2ter.HexJSON{Col: col, Row: row, Landform: hmz2ter.SaltWater}
			if col <= 5 {
				h.Landform = hmz2ter.Hills
				h.Elevation = &hmz2ter.ElevationJSON{Median: int16(100 * col)}
			}
			hexes = append(hexes, h)
		}
	}
	m := NewMap(g, hexes)
	r := DefaultRules()
	var h *hmz2ter.HexJSON
	for i := range m.Hexes {
		if m.Hexes[i].Col == 0 && m.Hexes[i].Row == 1 {
			h = &m.Hexes[i]
		}
	}
	p := m.Profile(h, 90, r)
	if p[0] != 0 || p[len(p)-1] != 0 {
		t.Fatalf("profile %v: want to start at the sea and end at the hex (0 m)", p)
	}
	for i := 2; i < len(p); i++ {
		if p[i] > p[i-1] {
			t.Fatalf("profile %v: should descend from the ridge at column 5 to the hex", p)
		}
	}
	// Each column is about 8.3 km wide, so six columns take about ten steps.
	if n := len(p) - 2; n < 9 || n > 11 {
		t.Errorf("profile %v: %d upwind samples, want about 10", p, n)
	}
	// Off the map counts as sea: a westerly trace ends within the hex's own
	// width (one 5 km step still lands in column 0).
	if p := m.Profile(h, 270, r); len(p) > 3 {
		t.Errorf("westerly profile %v: want the sea, at most one sample, and the hex", p)
	}
}

func TestWindOffsets(t *testing.T) {
	r := DefaultRules()
	for _, tc := range []struct {
		rays   int
		spread float64
		want   []float64
	}{
		{5, 20, []float64{-20, -10, 0, 10, 20}},
		{1, 20, []float64{0}},
		{2, 15, []float64{-15, 15}},
		{3, 0, []float64{0, 0, 0}},
	} {
		r.WindRays, r.WindSpreadDeg = tc.rays, tc.spread
		if got := r.WindOffsets(); !slices.Equal(got, tc.want) {
			t.Errorf("%d rays, spread %g: got %v, want %v", tc.rays, tc.spread, got, tc.want)
		}
	}
	for _, bad := range []struct {
		rays   int
		spread float64
	}{{0, 20}, {5, -1}} {
		r := DefaultRules()
		r.WindRays, r.WindSpreadDeg = bad.rays, bad.spread
		if r.Validate() == nil {
			t.Errorf("%d rays, spread %g: no error", bad.rays, bad.spread)
		}
	}
}

// TestFan checks that a single ray is the old single-line trace, and that a
// fan's result is the mean of its rays.
func TestFan(t *testing.T) {
	g, err := hmz2ele.NewGrid(48, 1000, 400)
	if err != nil {
		t.Fatal(err)
	}
	var hexes []hmz2ter.HexJSON
	for row := range g.Rows {
		for col := range g.Columns {
			if !g.Contains(col, row) {
				continue
			}
			h := hmz2ter.HexJSON{Col: col, Row: row, Landform: hmz2ter.SaltWater}
			if col <= 7 {
				h.Landform = hmz2ter.Hills
				h.Elevation = &hmz2ter.ElevationJSON{Median: int16(100*col + 37*(row%3))}
			}
			hexes = append(hexes, h)
		}
	}
	m := NewMap(g, hexes)
	h := &m.Hexes[slices.IndexFunc(m.Hexes, func(h hmz2ter.HexJSON) bool { return h.Col == 1 && h.Row == 2 })]
	r := DefaultRules()
	r.WindRays = 1
	mo, li := r.Moisture(m.Profile(h, 90, r))
	if fm, fl := m.Fan(h, 90, r); fm != mo || fl != li {
		t.Errorf("one ray: got %v, %v; want %v, %v", fm, fl, mo, li)
	}
	r = DefaultRules()
	var sm, sl float64
	for _, o := range r.WindOffsets() {
		m1, l1 := r.Moisture(m.Profile(h, 90+o, r))
		sm, sl = sm+m1, sl+l1
	}
	if fm, fl := m.Fan(h, 90, r); fm != sm/5 || fl != sl/5 {
		t.Errorf("five rays: got %v, %v; want %v, %v", fm, fl, sm/5, sl/5)
	}
}

func TestBiome(t *testing.T) {
	r := DefaultRules()
	c := func(lat, temp, precip, lift float64) Climate {
		span := r.RangeBase + r.RangePerDeg*math.Abs(lat)
		return Climate{Latitude: lat, Temperature: temp, Warmest: temp + span/2, Coldest: temp - span/2, Precipitation: precip, Lift: lift}
	}
	for _, tc := range []struct {
		name     string
		c        Climate
		landform hmz2ter.Landform
		want     hmz2ter.Biome
	}{
		{"ice", c(10, -4, 1000, 0), hmz2ter.Mountains, hmz2ter.BiomeClear},
		{"alpine", c(10, 7, 1000, 0), hmz2ter.Mountains, Alpine},
		{"tundra", c(70, -5, 300, 0), hmz2ter.Flats, Tundra},
		{"desert", c(20, 25, 300, 0), hmz2ter.Plains, Desert},
		{"scrubland", c(20, 25, 700, 0), hmz2ter.Plains, Scrubland},
		{"steppe", c(25, 12, 400, 0), hmz2ter.Hills, Steppe},
		{"rainforest", c(8, 26, 2500, 0), hmz2ter.Hills, TropicalRainforest},
		{"dry forest", c(8, 26, 1500, 0), hmz2ter.Hills, TropicalDryForest},
		{"savanna", c(8, 26, 1000, 0), hmz2ter.Plains, Savanna},
		{"cloud forest", c(10, 15, 1200, 150), hmz2ter.Mountains, CloudForest},
		{"leeward montane", c(10, 15, 1200, 50), hmz2ter.Mountains, TemperateForest},
		{"montane plains", c(10, 15, 1200, 150), hmz2ter.Plains, TemperateForest},
		{"subtropical montane", c(26, 12, 900, 0), hmz2ter.Hills, TemperateForest},
		{"boreal", c(45, 3, 800, 0), hmz2ter.Hills, BorealForest},
		{"temperate rainforest", c(45, 10, 2500, 0), hmz2ter.Hills, TemperateRainforest},
		{"grassland", c(45, 10, 600, 0), hmz2ter.Plains, Grassland},
		{"temperate forest", c(45, 10, 1000, 0), hmz2ter.Hills, TemperateForest},
	} {
		if got := r.Biome(tc.c, tc.landform); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestSurface(t *testing.T) {
	r := DefaultRules()
	hex := func(landform hmz2ter.Landform, median int16, flags ...hmz2ter.Flag) *hmz2ter.HexJSON {
		return &hmz2ter.HexJSON{Landform: landform, Flags: flags, Elevation: &hmz2ter.ElevationJSON{Median: median}}
	}
	warm := Climate{Latitude: 10, Temperature: 26, Warmest: 27, Coldest: 25, Precipitation: 2000}
	dry := Climate{Latitude: 20, Temperature: 25, Warmest: 28, Coldest: 22, Precipitation: 600}
	cool := Climate{Latitude: 45, Temperature: 8, Warmest: 16, Coldest: 0, Precipitation: 1000}
	for _, tc := range []struct {
		name  string
		c     Climate
		biome hmz2ter.Biome
		h     *hmz2ter.HexJSON
		want  hmz2ter.Surface
	}{
		{"hills never wet", warm, TropicalRainforest, hex(hmz2ter.Hills, 5, hmz2ter.FlatSurface, hmz2ter.Coast), hmz2ter.SurfaceClear},
		{"filled void", warm, TropicalRainforest, hex(hmz2ter.Flats, 95, hmz2ter.FlatSurface), hmz2ter.SurfaceClear},
		{"dry flats without a hint", warm, TropicalRainforest, hex(hmz2ter.Flats, 5), hmz2ter.SurfaceClear},
		{"high flats on a river", warm, TropicalRainforest, hex(hmz2ter.Flats, 11, hmz2ter.River), hmz2ter.SurfaceClear},
		{"plains on a river", warm, TropicalRainforest, hex(hmz2ter.Plains, 5, hmz2ter.River), hmz2ter.SurfaceClear},
		{"mangroves", warm, TropicalRainforest, hex(hmz2ter.Flats, 5, hmz2ter.Coast), Mangroves},
		{"coastal salt flats", dry, Scrubland, hex(hmz2ter.Flats, 5, hmz2ter.Coast), SaltFlats},
		{"inland salt flats", dry, Scrubland, hex(hmz2ter.Plains, 30, hmz2ter.FlatSurface), SaltFlats},
		{"swamps", warm, TropicalRainforest, hex(hmz2ter.Flats, 5, hmz2ter.River), Swamps},
		{"marshes", warm, Savanna, hex(hmz2ter.Flats, 5, hmz2ter.River), Marshes},
		{"bogs", cool, TemperateForest, hex(hmz2ter.Flats, 5, hmz2ter.FlatSurface), Bogs},
		{"cool coast", cool, TemperateForest, hex(hmz2ter.Flats, 5, hmz2ter.Coast), Bogs},
		{"ice", Climate{Warmest: -1}, hmz2ter.BiomeClear, hex(hmz2ter.Mountains, 3000), GlacialIce},
	} {
		if got := r.Surface(tc.c, tc.biome, tc.h); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := func(h hmz2ter.HexJSON) HexJSON { return HexJSON{HexJSON: h} }
	good := []HexJSON{
		ok(hmz2ter.HexJSON{Landform: hmz2ter.SaltWater, Surface: "clear", Biome: "clear", Depth: hmz2ter.Shallow, Flags: []hmz2ter.Flag{hmz2ter.Coast}}),
		ok(hmz2ter.HexJSON{Landform: hmz2ter.FreshWater, Surface: "clear", Biome: "clear"}),
		ok(hmz2ter.HexJSON{Landform: hmz2ter.Cliffs, Surface: "clear", Biome: "clear", Flags: []hmz2ter.Flag{hmz2ter.Impassable}}),
		ok(hmz2ter.HexJSON{Landform: hmz2ter.Flats, Surface: Mangroves, Biome: Savanna, Flags: []hmz2ter.Flag{hmz2ter.Coast, hmz2ter.FlatSurface}}),
		ok(hmz2ter.HexJSON{Landform: hmz2ter.Mountains, Surface: GlacialIce, Biome: "clear"}),
	}
	if err := Validate(good); err != nil {
		t.Errorf("valid hexes: %v", err)
	}
	for _, tc := range []struct {
		name string
		h    hmz2ter.HexJSON
		want string
	}{
		{"unset surface", hmz2ter.HexJSON{Landform: hmz2ter.Hills, Biome: Savanna}, `surface ""`},
		{"unset biome", hmz2ter.HexJSON{Landform: hmz2ter.Hills, Surface: "clear"}, `biome ""`},
		{"clear biome without ice", hmz2ter.HexJSON{Landform: hmz2ter.Hills, Surface: "clear", Biome: "clear"}, "if and only if"},
		{"ice with a biome", hmz2ter.HexJSON{Landform: hmz2ter.Hills, Surface: GlacialIce, Biome: Alpine}, "if and only if"},
		{"wetland on hills", hmz2ter.HexJSON{Landform: hmz2ter.Hills, Surface: Marshes, Biome: Savanna}, "only on flats and plains"},
		{"inland mangroves", hmz2ter.HexJSON{Landform: hmz2ter.Flats, Surface: Mangroves, Biome: Savanna}, "without the coast flag"},
		{"deep coast", hmz2ter.HexJSON{Landform: hmz2ter.SaltWater, Surface: "clear", Biome: "clear", Depth: hmz2ter.Deep, Flags: []hmz2ter.Flag{hmz2ter.Coast}}, "not shallow"},
		{"lake with a biome", hmz2ter.HexJSON{Landform: hmz2ter.FreshWater, Surface: "clear", Biome: Savanna}, "want clear and clear"},
		{"cliffs not impassable", hmz2ter.HexJSON{Landform: hmz2ter.Cliffs, Surface: "clear", Biome: "clear"}, "impassable"},
		{"river on salt water", hmz2ter.HexJSON{Landform: hmz2ter.SaltWater, Surface: "clear", Biome: "clear", Depth: hmz2ter.Open, Flags: []hmz2ter.Flag{hmz2ter.River}}, `flag "river"`},
		{"unset landform", hmz2ter.HexJSON{}, "unknown or unset landform"},
	} {
		err := Validate([]HexJSON{ok(tc.h)})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one containing %q", tc.name, err, tc.want)
		}
	}
}

// A point on a pixel edge, or a hair off it from rounding error, belongs to
// the pixel to its right or below, and so to that pixel's hex.
func TestAtPixelEdge(t *testing.T) {
	g, err := hmz2ele.NewGrid(48, 1000, 400)
	if err != nil {
		t.Fatal(err)
	}
	var hexes []hmz2ter.HexJSON
	for row := range g.Rows {
		for col := range g.Columns {
			if g.Contains(col, row) {
				hexes = append(hexes, hmz2ter.HexJSON{Col: col, Row: row})
			}
		}
	}
	m := NewMap(g, hexes)
	// In even column 2, y = 96 is the edge between hexes (2, 0) above and (2, 1) below.
	x, _ := g.Center(2, 1)
	for _, y := range []float64{96, 96 - 1e-12, 96 + 1e-12} {
		if h := m.At(x, y); h == nil || h.Col != 2 || h.Row != 1 {
			t.Errorf("At(%g, %.15g) = %+v, want hex (2, 1)", x, y, h)
		}
	}
}
