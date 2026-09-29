// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"errors"
	"fmt"
	"slices"

	"github.com/maloquacious/hmz2ter"
)

// maxErrors limits how many invalid hexes Validate reports.
const maxErrors = 20

// Validate checks every hex against the terrain model's valid combinations
// and returns the problems found, or nil. Once the climate step has run, an
// unset surface or biome is an error.
//
// The flat-surface flag is allowed on land although the model doesn't list
// it: it is hmz2ter's hint to this step, carried through for reference.
func Validate(hexes []HexJSON) error {
	var errs []error
	for _, h := range hexes {
		for _, msg := range check(&h.HexJSON) {
			if len(errs) == maxErrors {
				return errors.Join(append(errs, errors.New("more errors not shown"))...)
			}
			errs = append(errs, fmt.Errorf("hex (%d, %d) %s: %s", h.Col, h.Row, h.Landform, msg))
		}
	}
	return errors.Join(errs...)
}

func check(h *hmz2ter.HexJSON) []string {
	var bad []string
	flagsWithin := func(allowed ...hmz2ter.Flag) {
		for _, f := range h.Flags {
			if !slices.Contains(allowed, f) {
				bad = append(bad, fmt.Sprintf("flag %q not allowed", f))
			}
		}
	}
	clear := func() {
		if h.Surface != hmz2ter.SurfaceClear || h.Biome != hmz2ter.BiomeClear {
			bad = append(bad, fmt.Sprintf("surface %q and biome %q, want clear and clear", h.Surface, h.Biome))
		}
	}
	noDepth := func() {
		if h.Depth != hmz2ter.DepthNone {
			bad = append(bad, fmt.Sprintf("depth %q on a hex that isn't salt water", h.Depth))
		}
	}
	switch h.Landform {
	case hmz2ter.FreshWater:
		clear()
		noDepth()
		flagsWithin()
	case hmz2ter.SaltWater:
		clear()
		if !slices.Contains([]hmz2ter.Depth{hmz2ter.Shallow, hmz2ter.Open, hmz2ter.Deep}, h.Depth) {
			bad = append(bad, fmt.Sprintf("depth %q", h.Depth))
		}
		flagsWithin(hmz2ter.Coast, hmz2ter.InlandSea)
		if hasFlag(h, hmz2ter.Coast) && h.Depth != hmz2ter.Shallow {
			bad = append(bad, fmt.Sprintf("coast water is %q, not shallow", h.Depth))
		}
	case hmz2ter.Cliffs, hmz2ter.Badlands:
		clear()
		noDepth()
		flagsWithin(hmz2ter.Impassable)
		if !hasFlag(h, hmz2ter.Impassable) {
			bad = append(bad, "missing the impassable flag")
		}
	case hmz2ter.Flats, hmz2ter.Plains, hmz2ter.RollingPlains, hmz2ter.Hills, hmz2ter.Mountains, hmz2ter.Plateaus, hmz2ter.VolcanicHighlands:
		noDepth()
		flagsWithin(hmz2ter.Coast, hmz2ter.River, hmz2ter.Volcano, hmz2ter.Impassable, hmz2ter.FlatSurface)
		if !slices.Contains(Surfaces, h.Surface) {
			bad = append(bad, fmt.Sprintf("surface %q", h.Surface))
		}
		if !slices.Contains(Biomes, h.Biome) {
			bad = append(bad, fmt.Sprintf("biome %q", h.Biome))
		}
		if (h.Biome == hmz2ter.BiomeClear) != (h.Surface == GlacialIce) {
			bad = append(bad, fmt.Sprintf("biome %q with surface %q: clear if and only if glacial-ice", h.Biome, h.Surface))
		}
		switch h.Surface {
		case Marshes, Swamps, Bogs, Mangroves, SaltFlats:
			if h.Landform != hmz2ter.Flats && h.Landform != hmz2ter.Plains {
				bad = append(bad, fmt.Sprintf("%s only on flats and plains", h.Surface))
			}
		}
		if h.Surface == Mangroves && !hasFlag(h, hmz2ter.Coast) {
			bad = append(bad, "mangroves without the coast flag")
		}
	default:
		bad = append(bad, "unknown or unset landform")
	}
	return bad
}
