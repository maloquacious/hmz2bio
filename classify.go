// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"math"
	"slices"

	"github.com/maloquacious/hmz2ter"
)

// Surfaces beyond hmz2ter's clear.
const (
	GlacialIce hmz2ter.Surface = "glacial-ice"
	Marshes    hmz2ter.Surface = "marshes"
	Swamps     hmz2ter.Surface = "swamps"
	Bogs       hmz2ter.Surface = "bogs"
	Mangroves  hmz2ter.Surface = "mangroves"
	SaltFlats  hmz2ter.Surface = "salt-flats"
)

// Biomes beyond hmz2ter's clear.
const (
	Tundra              hmz2ter.Biome = "tundra"
	Alpine              hmz2ter.Biome = "alpine"
	Desert              hmz2ter.Biome = "desert"
	Scrubland           hmz2ter.Biome = "scrubland"
	Grassland           hmz2ter.Biome = "grassland"
	Steppe              hmz2ter.Biome = "steppe"
	Savanna             hmz2ter.Biome = "savanna"
	BorealForest        hmz2ter.Biome = "boreal-forest"
	TemperateForest     hmz2ter.Biome = "temperate-forest"
	TemperateRainforest hmz2ter.Biome = "temperate-rainforest"
	TropicalDryForest   hmz2ter.Biome = "tropical-dry-forest"
	TropicalRainforest  hmz2ter.Biome = "tropical-rainforest"
	CloudForest         hmz2ter.Biome = "cloud-forest"
)

// Surfaces and Biomes list every value in the terrain model, in order.
var (
	Surfaces = []hmz2ter.Surface{hmz2ter.SurfaceClear, GlacialIce, Marshes, Swamps, Bogs, Mangroves, SaltFlats}
	Biomes   = []hmz2ter.Biome{hmz2ter.BiomeClear, Tundra, Alpine, Desert, Scrubland, Grassland, Steppe, Savanna,
		BorealForest, TemperateForest, TemperateRainforest, TropicalDryForest, TropicalRainforest, CloudForest}
)

// IsForest reports whether the biome is a forest.
func IsForest(b hmz2ter.Biome) bool {
	return slices.Contains([]hmz2ter.Biome{BorealForest, TemperateForest, TemperateRainforest, TropicalDryForest, TropicalRainforest, CloudForest}, b)
}

// Arid returns the precipitation below which a hex of mean temperature t is
// dry.
func (r Rules) Arid(t float64) float64 {
	return r.AridPerDeg*t + r.AridBase
}

// Biome returns a land hex's biome, or clear if it is glacial ice.
func (r Rules) Biome(c Climate, landform hmz2ter.Landform) hmz2ter.Biome {
	span := r.RangeBase + r.RangePerDeg*math.Abs(c.Latitude)
	seaLevel := r.SeaLevelTemp.At(c.Latitude)
	switch {
	case c.Warmest < r.IceBelow:
		return hmz2ter.BiomeClear
	case c.Warmest < r.TreeLineBelow:
		if seaLevel+span/2 < r.TreeLineBelow {
			return Tundra
		}
		return Alpine
	}
	t, p, arid := c.Temperature, c.Precipitation, r.Arid(c.Temperature)
	switch {
	case p < r.DesertShare*arid:
		return Desert
	case p < arid && t >= r.HotMin:
		return Scrubland
	case p < arid:
		return Steppe
	}
	if t < r.MontaneBelow {
		switch landform {
		case hmz2ter.Hills, hmz2ter.Mountains, hmz2ter.Plateaus, hmz2ter.VolcanicHighlands:
			if seaLevel-span/2 >= r.CloudForestColdestMin && c.Lift >= r.CloudForestLift && p >= r.CloudForestMin {
				return CloudForest
			}
		}
		if seaLevel-span/2 >= r.TropicsColdestMin {
			return TemperateForest
		}
	}
	switch {
	case t >= r.HotMin && p >= r.RainforestMin:
		return TropicalRainforest
	case t >= r.HotMin && p >= r.DryForestMin:
		return TropicalDryForest
	case t >= r.HotMin:
		return Savanna
	case t < r.BorealBelow:
		return BorealForest
	case p >= r.RainforestMin:
		return TemperateRainforest
	case p < r.GrasslandAridity*arid:
		return Grassland
	}
	return TemperateForest
}

// Surface returns a land hex's surface, given its climate and biome.
func (r Rules) Surface(c Climate, biome hmz2ter.Biome, h *hmz2ter.HexJSON) hmz2ter.Surface {
	if c.Warmest < r.IceBelow {
		return GlacialIce
	}
	if !r.Wet(h) {
		return hmz2ter.SurfaceClear
	}
	switch {
	case c.Precipitation < r.Arid(c.Temperature):
		return SaltFlats
	case hasFlag(h, hmz2ter.Coast) && c.Coldest >= r.MangroveColdestMin:
		return Mangroves
	case c.Temperature < r.BogsBelow:
		return Bogs
	case IsForest(biome):
		return Swamps
	}
	return Marshes
}

// Wet reports whether a land hex can hold a wetland, mangrove, or salt-flat
// surface: flats or plains marked flat-surface, low enough to be more than
// a filled void in the source, or low flats on a coast or river.
func (r Rules) Wet(h *hmz2ter.HexJSON) bool {
	if (h.Landform != hmz2ter.Flats && h.Landform != hmz2ter.Plains) || h.Elevation == nil {
		return false
	}
	median := int(h.Elevation.Median)
	if hasFlag(h, hmz2ter.FlatSurface) && median <= r.FlatSurfaceMax {
		return true
	}
	return h.Landform == hmz2ter.Flats && median <= r.LowlandMax && (hasFlag(h, hmz2ter.Coast) || hasFlag(h, hmz2ter.River))
}

func hasFlag(h *hmz2ter.HexJSON, f hmz2ter.Flag) bool {
	return slices.Contains(h.Flags, f)
}
