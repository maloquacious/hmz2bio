// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"github.com/maloquacious/hmz2ter"
)

// Document is the climate output: every field of hmz2ter's terrain, plus
// the climate step's own. Hexes replaces the terrain's hexes with ones that
// carry their climate.
type Document struct {
	Hmz2bioVersion string      `json:"hmz2bio_version"`
	TerrainFile    TerrainInfo `json:"terrain"`
	hmz2ter.Terrain
	ClimateRules  Rules      `json:"climate_rules"`
	ClimateMethod MethodInfo `json:"climate_method"`
	ClimateStats  Stats      `json:"climate_stats"`
	Hexes         []HexJSON  `json:"hexes"`
}

// TerrainInfo identifies the hmz2ter file the climate was built from.
type TerrainInfo struct {
	FileName string `json:"file_name"`
}

// MethodInfo describes the parts of the model that Rules doesn't hold.
type MethodInfo struct {
	Latitude      string `json:"latitude"`
	Temperature   string `json:"temperature"`
	Precipitation string `json:"precipitation"`
	Biome         string `json:"biome"`
	Surface       string `json:"surface"`
}

// Method is the model's description, as written to JSON.
var Method = MethodInfo{
	Latitude:      "interpolated linearly from top_lat_deg at the raster's top edge to bottom_lat_deg at its bottom edge, at the hex center's y",
	Temperature:   "sea_level_temp_c at the latitude, less lapse_rate_c_per_km per km of median elevation; warmest and coldest months are the mean ± (annual_range_base_c + annual_range_per_deg_c × |latitude|) / 2",
	Precipitation: "windward_precip_mm × (convective_share + (1 − convective_share) × moisture × (1 + lift_gain_per_km × lift_m / 1000)); moisture starts at 1 over the sea (salt water, cliffs, badlands, or off the map) and each step_km upwind keeps exp(−step_km/rainout_km − rise/orographic_m); lift is the median elevation above the lowest point within lift_window_km upwind; each wind is traced along wind_rays bearings evenly spaced across ± wind_spread_deg of its bearing (one ray: the bearing itself), and moisture and lift are the means over the rays; the trades and westerlies are traced separately and blended by latitude; distances are campaign km, elevations real m",
	Biome:         "warmest month below ice → clear (glacial ice); below the tree line → tundra if the sea-level warmest month is also below it, else alpine; precipitation below arid_per_deg_mm × T + arid_base_mm → desert (under desert_share of it), scrubland (hot), or steppe; cooler than montane_below_c → cloud forest on hills, mountains, plateaus, and volcanic highlands where the sea-level coldest month is at least cloud_forest_coldest_min_c, lift at least cloud_forest_lift_m, and precipitation at least cloud_forest_min_mm, else temperate forest in the tropics (sea-level coldest month ≥ tropics_coldest_min_c); hot → tropical rainforest, tropical dry forest, or savanna by precipitation; otherwise boreal forest, temperate rainforest, grassland, or temperate forest",
	Surface:       "glacial ice with the biome; otherwise only flats and plains that are flat-surface with a median of at most flat_surface_max_m, or flats on a coast or river with a median of at most lowland_max_m, get a surface: salt flats if dry (below the aridity limit), mangroves on the coast if the coldest month is at least mangrove_coldest_min_c, bogs if cooler than bogs_below_c, swamps in a forest biome, marshes otherwise; the rest are clear",
}

// HexJSON is one hex: hmz2ter's hex, with its surface and biome set, plus
// the climate of a land hex.
type HexJSON struct {
	hmz2ter.HexJSON
	Climate *Climate `json:"climate,omitempty"`
}

// Stats counts land hexes by biome and surface.
type Stats struct {
	LandHexes int                     `json:"land_hexes"`
	Biomes    map[hmz2ter.Biome]int   `json:"biomes"`
	Surfaces  map[hmz2ter.Surface]int `json:"surfaces"`
}

// Build gives every land hex its climate, biome, and surface. Other hexes
// are copied unchanged.
func Build(m *Map, r Rules) []HexJSON {
	out := make([]HexJSON, len(m.Hexes))
	for i := range m.Hexes {
		h := &m.Hexes[i]
		out[i].HexJSON = *h
		if !h.Landform.IsLand() {
			continue
		}
		c := m.Climate(h, r)
		biome := r.Biome(c, h.Landform)
		out[i].Biome, out[i].Surface, out[i].Climate = biome, r.Surface(c, biome, h), &c
	}
	return out
}

// Summarize counts the land hexes.
func Summarize(hexes []HexJSON) Stats {
	s := Stats{Biomes: map[hmz2ter.Biome]int{}, Surfaces: map[hmz2ter.Surface]int{}}
	for _, h := range hexes {
		if !h.Landform.IsLand() {
			continue
		}
		s.LandHexes++
		s.Biomes[h.Biome]++
		s.Surfaces[h.Surface]++
	}
	return s
}
