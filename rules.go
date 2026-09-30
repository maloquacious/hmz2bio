// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"fmt"
	"math"
	"slices"
)

// Table is a piecewise-linear function of latitude: each entry is
// [degrees from the equator, value], in increasing latitude. Values are
// interpolated between entries and held flat beyond the ends.
type Table [][2]float64

// At returns the table's value at the latitude, north or south.
func (t Table) At(lat float64) float64 {
	x := math.Abs(lat)
	if x <= t[0][0] {
		return t[0][1]
	}
	for i := 1; i < len(t); i++ {
		if x <= t[i][0] {
			x0, y0, x1, y1 := t[i-1][0], t[i-1][1], t[i][0], t[i][1]
			return y0 + (y1-y0)*(x-x0)/(x1-x0)
		}
	}
	return t[len(t)-1][1]
}

func (t Table) validate(name string) error {
	if len(t) == 0 {
		return fmt.Errorf("%s: empty table", name)
	}
	for i := 1; i < len(t); i++ {
		if t[i][0] <= t[i-1][0] {
			return fmt.Errorf("%s: latitudes must increase", name)
		}
	}
	return nil
}

// Rules holds the climate model's settings and the thresholds that choose
// biomes and surfaces. Temperatures are in °C and precipitation in mm per
// year. Elevations are real meters, as in the source. Distances along the
// wind are campaign kilometers: the air crosses the campaign world, which is
// about 3.4 times the real one, while lift depends only on height gained.
type Rules struct {
	// TopLat and BottomLat are the latitudes of the raster's top and bottom
	// edges, in degrees, north positive. A hex's latitude is interpolated
	// from its center's y.
	TopLat    float64 `json:"top_lat_deg"`
	BottomLat float64 `json:"bottom_lat_deg"`

	// SeaLevelTemp is the mean annual temperature at sea level by latitude.
	// LapseRate cools it with the hex's median elevation.
	SeaLevelTemp Table   `json:"sea_level_temp_c"`
	LapseRate    float64 `json:"lapse_rate_c_per_km"`
	// The annual range, warmest month minus coldest, is RangeBase +
	// RangePerDeg × |latitude|, split evenly about the mean.
	RangeBase   float64 `json:"annual_range_base_c"`
	RangePerDeg float64 `json:"annual_range_per_deg_c"`

	// WindwardPrecip is the annual precipitation on a windward coast at
	// sea level, by latitude. ConvectiveShare is the part of it that falls
	// regardless of the wind (the tropical wet season's storms); the rest
	// depends on the moisture the wind brings.
	WindwardPrecip  Table `json:"windward_precip_mm"`
	ConvectiveShare Table `json:"convective_share"`

	// The prevailing wind blows from TradesFrom (a bearing, clockwise from
	// north) at latitudes up to TradesMaxLat and from WesterliesFrom beyond
	// WesterliesMinLat, blended linearly in between.
	TradesFrom       float64 `json:"trades_from_deg"`
	WesterliesFrom   float64 `json:"westerlies_from_deg"`
	TradesMaxLat     float64 `json:"trades_max_lat_deg"`
	WesterliesMinLat float64 `json:"westerlies_min_lat_deg"`
	// Each wind is traced along WindRays bearings, evenly spaced from
	// WindSpreadDeg on one side of its bearing to WindSpreadDeg on the other
	// (a single ray follows the bearing itself), and a hex's moisture and
	// lift are the means over the rays. A fan, rather than one ray, keeps a
	// single peak or gap upwind from striping the map along the wind.
	WindRays      int     `json:"wind_rays"`
	WindSpreadDeg float64 `json:"wind_spread_deg"`

	// Moisture: air leaves the sea saturated (1) and is traced upwind in
	// steps of StepKm, for at most ReachKm. Each step keeps
	// exp(−StepKm/RainoutKm − rise/OrographicM) of its moisture, where rise
	// is the height gained, so mountains wring the air dry and leave a rain
	// shadow behind them.
	StepKm      float64 `json:"step_km"`
	ReachKm     float64 `json:"reach_km"`
	RainoutKm   float64 `json:"rainout_km"`
	OrographicM float64 `json:"orographic_m"`
	// Lift is the hex's height above the lowest point within LiftWindowKm
	// upwind. Rain on the hex is multiplied by 1 + LiftGain × lift / 1000.
	LiftWindowKm float64 `json:"lift_window_km"`
	LiftGain     float64 `json:"lift_gain_per_km"`

	// A hex whose warmest month is below IceBelow is glacial ice, and below
	// TreeLineBelow is above the tree line: tundra if the warmest month
	// would be below TreeLineBelow at sea level at that latitude, and
	// alpine if not.
	IceBelow      float64 `json:"ice_warmest_below_c"`
	TreeLineBelow float64 `json:"tree_line_warmest_below_c"`

	// Aridity (Köppen): a hex is dry if its precipitation is below
	// AridPerDeg × T + AridBase, where T is its mean temperature, and a
	// desert below DesertShare of that. A dry hex at least HotMin is
	// scrubland, and steppe if cooler.
	AridPerDeg  float64 `json:"arid_per_deg_mm"`
	AridBase    float64 `json:"arid_base_mm"`
	DesertShare float64 `json:"desert_share"`
	HotMin      float64 `json:"hot_min_c"`

	// A humid hex cooler than MontaneBelow is montane. It is cloud forest
	// where the coldest month at sea level is at least
	// CloudForestColdestMin (the tropics and subtropics), on hills,
	// mountains, plateaus, or volcanic highlands, on a windward slope (lift
	// of at least CloudForestLift) with at least CloudForestMin. Otherwise,
	// in the tropics, where the coldest month at sea level is at least
	// TropicsColdestMin, it is temperate forest.
	MontaneBelow          float64 `json:"montane_below_c"`
	CloudForestColdestMin float64 `json:"cloud_forest_coldest_min_c"`
	CloudForestLift       float64 `json:"cloud_forest_lift_m"`
	CloudForestMin        float64 `json:"cloud_forest_min_mm"`
	TropicsColdestMin     float64 `json:"tropics_coldest_min_c"`
	// A humid hex at least HotMin, tropical or not, is tropical rainforest
	// with at least RainforestMin, tropical dry forest with at least
	// DryForestMin, and savanna with less.
	RainforestMin float64 `json:"rainforest_min_mm"`
	DryForestMin  float64 `json:"dry_forest_min_mm"`
	// A humid hex cooler than HotMin outside the tropics is boreal forest
	// below BorealBelow, temperate rainforest with at least RainforestMin,
	// grassland below GrasslandAridity × the aridity limit, and temperate
	// forest otherwise.
	BorealBelow      float64 `json:"boreal_below_c"`
	GrasslandAridity float64 `json:"grassland_aridity"`

	// Wetland surfaces (wetlands, mangroves, and salt flats) go only on
	// flats and plains that hmz2ter marked flat-surface with a median
	// elevation of at most FlatSurfaceMax, or on flats with the coast or
	// river flag and a median of at most LowlandMax.
	FlatSurfaceMax int `json:"flat_surface_max_m"`
	LowlandMax     int `json:"lowland_max_m"`
	// Such a hex is salt flats if it is dry (see AridBase), mangroves on
	// the coast if its coldest month is at least MangroveColdestMin, bogs
	// below BogsBelow, swamps in a forest biome, and marshes otherwise.
	MangroveColdestMin float64 `json:"mangrove_coldest_min_c"`
	BogsBelow          float64 `json:"bogs_below_c"`
}

// DefaultRules returns the campaign's rules.
func DefaultRules() Rules {
	return Rules{
		TopLat:    27,
		BottomLat: 7,

		SeaLevelTemp: Table{{0, 27}, {10, 27}, {15, 26.5}, {20, 25}, {25, 22.5}, {30, 20}, {35, 17}, {40, 14.5}, {45, 12}, {50, 8}, {55, 4}, {60, 0}, {70, -8}, {80, -16}, {90, -22}},
		LapseRate:    6.5,
		RangeBase:    1,
		RangePerDeg:  0.33,

		WindwardPrecip:  Table{{0, 3100}, {5, 3300}, {10, 2700}, {15, 2000}, {20, 1400}, {25, 1000}, {30, 850}, {35, 1000}, {40, 1250}, {45, 1400}, {50, 1400}, {55, 1250}, {60, 1000}, {70, 500}, {80, 250}, {90, 150}},
		ConvectiveShare: Table{{0, 0.5}, {10, 0.45}, {20, 0.3}, {30, 0.2}},

		TradesFrom:       60,
		WesterliesFrom:   255,
		TradesMaxLat:     27,
		WesterliesMinLat: 33,
		WindRays:         5,
		WindSpreadDeg:    20,

		StepKm:       5,
		ReachKm:      1000,
		RainoutKm:    800,
		OrographicM:  1500,
		LiftWindowKm: 30,
		LiftGain:     1.5,

		IceBelow:      0,
		TreeLineBelow: 10,

		AridPerDeg:  20,
		AridBase:    280,
		DesertShare: 0.5,
		HotMin:      18,

		MontaneBelow:          18,
		CloudForestColdestMin: 15,
		CloudForestLift:       100,
		CloudForestMin:        1000,
		TropicsColdestMin:     18,
		RainforestMin:         2000,
		DryForestMin:          1200,
		BorealBelow:           4,
		GrasslandAridity:      1.5,

		FlatSurfaceMax:     60,
		LowlandMax:         10,
		MangroveColdestMin: 15,
		BogsBelow:          10,
	}
}

// Validate reports settings the model can't use.
func (r Rules) Validate() error {
	for name, t := range map[string]Table{"sea_level_temp_c": r.SeaLevelTemp, "windward_precip_mm": r.WindwardPrecip, "convective_share": r.ConvectiveShare} {
		if err := t.validate(name); err != nil {
			return err
		}
	}
	if r.TopLat < -90 || r.TopLat > 90 || r.BottomLat < -90 || r.BottomLat > 90 {
		return fmt.Errorf("latitudes %g and %g: must be within ±90°", r.TopLat, r.BottomLat)
	}
	if r.TopLat == r.BottomLat {
		return fmt.Errorf("top and bottom latitudes are both %g°", r.TopLat)
	}
	if r.WesterliesMinLat < r.TradesMaxLat {
		return fmt.Errorf("westerlies start at %g°, before the trades end at %g°", r.WesterliesMinLat, r.TradesMaxLat)
	}
	if r.WindRays < 1 {
		return fmt.Errorf("wind rays %d: must be at least 1", r.WindRays)
	}
	if r.WindSpreadDeg < 0 {
		return fmt.Errorf("wind spread %g°: must be at least 0", r.WindSpreadDeg)
	}
	if slices.ContainsFunc([]float64{r.StepKm, r.ReachKm, r.RainoutKm, r.OrographicM}, func(v float64) bool { return v <= 0 }) {
		return fmt.Errorf("step, reach, rainout, and orographic settings must be positive")
	}
	return nil
}

// Latitude returns the latitude at raster row y of a raster that is height
// pixels tall.
func (r Rules) Latitude(y float64, height int) float64 {
	return r.TopLat + (r.BottomLat-r.TopLat)*y/float64(height)
}

// Wind returns the bearings the wind blows from at the latitude, as trades
// and westerlies, and the westerlies' weight, from 0 to 1.
func (r Rules) Wind(lat float64) (trades, westerlies, weight float64) {
	trades, westerlies = r.TradesFrom, r.WesterliesFrom
	if lat < 0 {
		// Mirror north–south: north-east trades become south-east trades.
		trades, westerlies = 180-trades, 180-westerlies
	}
	switch x := math.Abs(lat); {
	case x <= r.TradesMaxLat:
		weight = 0
	case x >= r.WesterliesMinLat:
		weight = 1
	default:
		weight = (x - r.TradesMaxLat) / (r.WesterliesMinLat - r.TradesMaxLat)
	}
	return trades, westerlies, weight
}

// WindOffsets returns the offsets, in degrees, of a wind's rays from its
// bearing: WindRays values evenly spaced from −WindSpreadDeg to
// +WindSpreadDeg, or just 0 for a single ray.
func (r Rules) WindOffsets() []float64 {
	if r.WindRays == 1 {
		return []float64{0}
	}
	offsets := make([]float64, r.WindRays)
	for i := range offsets {
		offsets[i] = -r.WindSpreadDeg + 2*r.WindSpreadDeg*float64(i)/float64(r.WindRays-1)
	}
	return offsets
}
