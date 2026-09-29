// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2bio

import (
	"image"
	"image/color"
	"math"

	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

var (
	biomeColors = map[hmz2ter.Biome]color.RGBA{
		Tundra:              {0x9f, 0xb3, 0xa6, 0xff},
		Alpine:              {0xbd, 0xb6, 0xad, 0xff},
		Desert:              {0xf2, 0xe2, 0xa8, 0xff},
		Scrubland:           {0xc9, 0xa8, 0x6a, 0xff},
		Grassland:           {0xa9, 0xcf, 0x6e, 0xff},
		Steppe:              {0xd6, 0xcf, 0x98, 0xff},
		Savanna:             {0xd9, 0xb4, 0x4a, 0xff},
		BorealForest:        {0x3f, 0x6a, 0x5a, 0xff},
		TemperateForest:     {0x4f, 0x8f, 0x3a, 0xff},
		TemperateRainforest: {0x2f, 0x6f, 0x4f, 0xff},
		TropicalDryForest:   {0x8f, 0xaa, 0x3c, 0xff},
		TropicalRainforest:  {0x1f, 0x5f, 0x1f, 0xff},
		CloudForest:         {0x5a, 0x8f, 0x9a, 0xff},
	}
	surfaceColors = map[hmz2ter.Surface]color.RGBA{
		GlacialIce: {0xe8, 0xf4, 0xff, 0xff},
		Marshes:    {0x7a, 0xd0, 0xc0, 0xff},
		Swamps:     {0x3c, 0x8c, 0x7c, 0xff},
		Bogs:       {0x8a, 0x7a, 0x9a, 0xff},
		Mangroves:  {0x00, 0xa0, 0x80, 0xff},
		SaltFlats:  {0xff, 0xf0, 0xf5, 0xff},
	}
	depthColors = map[hmz2ter.Depth]color.RGBA{
		hmz2ter.Shallow: {0x4f, 0x93, 0xc4, 0xff},
		hmz2ter.Open:    {0x2b, 0x57, 0x8c, 0xff},
		hmz2ter.Deep:    {0x14, 0x2c, 0x55, 0xff},
	}
	freshWaterColor = color.RGBA{0x1f, 0x78, 0xd1, 0xff}
	coastWaterColor = color.RGBA{0x8e, 0xc9, 0xe8, 0xff}
	inlandSeaColor  = color.RGBA{0x2a, 0xa0, 0x98, 0xff}
	cliffsColor     = color.RGBA{0xc0, 0x10, 0x10, 0xff}
	badlandsColor   = color.RGBA{0x70, 0x20, 0x40, 0xff}
	volcanoColor    = color.RGBA{0xff, 0x00, 0xc8, 0xff}
	riverColor      = color.RGBA{0x10, 0x3a, 0xc8, 0xff}
)

// hexColor returns the preview color of a hex: land by its surface if it
// has one, and by its biome if not; water and border cuts as in hmz2ter's
// preview; volcanoes magenta.
func hexColor(h *HexJSON) color.RGBA {
	var c color.RGBA
	switch h.Landform {
	case hmz2ter.FreshWater:
		c = freshWaterColor
	case hmz2ter.SaltWater:
		c = depthColors[h.Depth]
		if hasFlag(&h.HexJSON, hmz2ter.Coast) {
			c = coastWaterColor
		}
		if hasFlag(&h.HexJSON, hmz2ter.InlandSea) {
			c = inlandSeaColor
		}
	case hmz2ter.Cliffs:
		c = cliffsColor
	case hmz2ter.Badlands:
		c = badlandsColor
	default:
		c = biomeColors[h.Biome]
		if s, ok := surfaceColors[h.Surface]; ok {
			c = s
		}
		if hasFlag(&h.HexJSON, hmz2ter.Volcano) {
			c = volcanoColor
		}
	}
	return c
}

// RenderPreview draws the hmz2ele preview, then fills each hex with its
// color (see hexColor), keeping the hex outlines. Rivers are drawn along hex
// edges.
func RenderPreview(g hmz2ele.Grid, hexes []HexJSON, rivers []hmz2riv.EdgeJSON, scale int) (*image.RGBA, error) {
	centers := make([]hmz2ele.Hex, len(hexes))
	for i, h := range hexes {
		centers[i] = hmz2ele.Hex{Col: h.Col, Row: h.Row, Center: h.Center}
	}
	img, err := hmz2ele.RenderPreview(g, centers, scale)
	if err != nil {
		return nil, err
	}
	fill := make(map[int]color.RGBA, len(hexes))
	for i := range hexes {
		fill[hexes[i].Row*g.Columns+hexes[i].Col] = hexColor(&hexes[i])
	}
	b := img.Bounds()
	owner := make([]int, b.Dx()*b.Dy())
	for py := range b.Dy() {
		for px := range b.Dx() {
			col, row := g.HexAt((float64(px)+0.5)*float64(scale), (float64(py)+0.5)*float64(scale))
			owner[py*b.Dx()+px] = -1
			if g.Contains(col, row) {
				owner[py*b.Dx()+px] = row*g.Columns + col
			}
		}
	}
	for py := range b.Dy() {
		for px := range b.Dx() {
			s := owner[py*b.Dx()+px]
			c, ok := fill[s]
			if s < 0 || !ok {
				continue
			}
			// Keep hmz2ele's outlines: an outline pixel borders another hex
			// to its east or south.
			if (px+1 < b.Dx() && owner[py*b.Dx()+px+1] != s) || (py+1 < b.Dy() && owner[(py+1)*b.Dx()+px] != s) {
				c = darken(c)
			}
			img.SetRGBA(px, py, c)
		}
	}
	for _, e := range rivers {
		x1, y1 := g.Vertex(hmz2ele.VertexKey{Col: e.From.Col, Row: e.From.Row, Corner: e.From.Corner})
		x2, y2 := g.Vertex(hmz2ele.VertexKey{Col: e.To.Col, Row: e.To.Row, Corner: e.To.Corner})
		drawLine(img, x1/float64(scale), y1/float64(scale), x2/float64(scale), y2/float64(scale), riverColor)
	}
	return img, nil
}

func darken(c color.RGBA) color.RGBA {
	scale := func(v uint8) uint8 { return uint8(uint16(v) * 3 / 4) }
	return color.RGBA{R: scale(c.R), G: scale(c.G), B: scale(c.B), A: c.A}
}

// drawLine draws a one-pixel line.
func drawLine(img *image.RGBA, x1, y1, x2, y2 float64, c color.RGBA) {
	steps := int(math.Ceil(max(math.Abs(x2-x1), math.Abs(y2-y1)))) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		p := image.Point{int(x1 + t*(x2-x1)), int(y1 + t*(y2-y1))}
		if p.In(img.Rect) {
			img.SetRGBA(p.X, p.Y, c)
		}
	}
}
