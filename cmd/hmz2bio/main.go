// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command hmz2bio gives every land hex of an hmz2ter terrain file a climate,
// a biome, and a surface, and writes the result as JSON, with an optional
// PNG preview.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/maloquacious/hmz2bio"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hmz2bio: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	rules := hmz2bio.DefaultRules()
	fs := flag.NewFlagSet("hmz2bio", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hmz2bio [flags] <terrain.json>\n\n")
		fs.PrintDefaults()
	}
	fs.Float64Var(&rules.TopLat, "top-lat", rules.TopLat, "latitude of the map's top edge, in degrees (north positive)")
	fs.Float64Var(&rules.BottomLat, "bottom-lat", rules.BottomLat, "latitude of the map's bottom edge, in degrees (north positive)")
	riversFile := fs.String("rivers", "", "hmz2riv JSON file, to draw rivers on the preview (optional)")
	output := fs.String("output", "", "JSON file to write (required)")
	preview := fs.String("preview", "", "PNG preview file to write (optional)")
	previewScale := fs.Int("preview-scale", 8, "raster pixels per preview pixel, in each direction")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, hmz2bio.Version())
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one input file, got %d", fs.NArg())
	}
	if *output == "" {
		return fmt.Errorf("-output is required")
	}
	if err := rules.Validate(); err != nil {
		return err
	}
	input := fs.Arg(0)
	start := time.Now()
	phase := func(name string) {
		fmt.Fprintf(stdout, "%-24s %6.1fs\n", name, time.Since(start).Seconds())
	}

	var terrain hmz2ter.Terrain
	if err := readJSON(input, &terrain); err != nil {
		return err
	}
	var rivers hmz2riv.Rivers
	if *riversFile != "" {
		if err := readJSON(*riversFile, &rivers); err != nil {
			return err
		}
	}
	phase("read input")
	md := terrain.Heightmap.Metadata
	g, err := hmz2ele.NewGrid(terrain.Grid.ApothemPx, int(md.Width), int(md.Height))
	if err != nil {
		return err
	}
	if g.Columns != terrain.Grid.Columns || g.Rows != terrain.Grid.Rows || len(terrain.Hexes) != terrain.Grid.HexCount {
		return fmt.Errorf("%s: grid is %d × %d with %d hexes, but the heightmap and apothem give %d × %d",
			input, terrain.Grid.Columns, terrain.Grid.Rows, len(terrain.Hexes), g.Columns, g.Rows)
	}
	for _, h := range terrain.Hexes {
		if !g.Contains(h.Col, h.Row) {
			return fmt.Errorf("%s: hex (%d, %d) is outside the grid", input, h.Col, h.Row)
		}
	}
	if *riversFile != "" && (rivers.Grid.ApothemPx != g.Apothem || rivers.Grid.Columns != g.Columns || rivers.Grid.Rows != g.Rows) {
		return fmt.Errorf("%s: grid doesn't match %s", *riversFile, input)
	}

	hexes := hmz2bio.Build(hmz2bio.NewMap(g, terrain.Hexes), rules)
	phase("climate")
	if err := hmz2bio.Validate(hexes); err != nil {
		return fmt.Errorf("invalid terrain:\n%w", err)
	}
	phase("validate")

	summary := hmz2bio.Summarize(hexes)
	terrain.Hexes = nil
	doc := hmz2bio.Document{
		Hmz2bioVersion: hmz2bio.Version().String(),
		TerrainFile:    hmz2bio.TerrainInfo{FileName: filepath.Base(input)},
		Terrain:        terrain,
		ClimateRules:   rules,
		ClimateMethod:  hmz2bio.Method,
		ClimateStats:   summary,
		Hexes:          hexes,
	}
	if err := writeFile(*output, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}); err != nil {
		return err
	}
	if *preview != "" {
		img, err := hmz2bio.RenderPreview(g, hexes, rivers.Edges, *previewScale)
		if err != nil {
			return err
		}
		if err := writeFile(*preview, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
			return err
		}
	}
	phase("write output")

	fmt.Fprintf(stdout, "land hexes:         %d\n", summary.LandHexes)
	fmt.Fprintf(stdout, "biomes:\n")
	for _, b := range hmz2bio.Biomes {
		if n := summary.Biomes[b]; n > 0 {
			fmt.Fprintf(stdout, "  %-22s %5d  %5.1f%%\n", b+":", n, 100*float64(n)/float64(summary.LandHexes))
		}
	}
	fmt.Fprintf(stdout, "surfaces:\n")
	for _, s := range hmz2bio.Surfaces {
		if n := summary.Surfaces[s]; n > 0 {
			fmt.Fprintf(stdout, "  %-22s %5d  %5.1f%%\n", s+":", n, 100*float64(n)/float64(summary.LandHexes))
		}
	}
	return nil
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := write(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	return f.Close()
}
