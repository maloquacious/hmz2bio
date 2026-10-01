# hmz2bio

`hmz2bio` is the climate step of the campaign-map pipeline.
It reads an [`hmz2ter`](https://github.com/maloquacious/hmz2ter) terrain file and gives every land hex a climate, a **biome**, and a **surface**, completing the terrain model's three layers (landform, surface, biome).
It writes the result as JSON, with an optional PNG preview.

The model is deliberately simple and explainable: mean annual temperature from latitude and elevation, annual precipitation from latitude and the moisture the prevailing wind brings over the relief, and a biome from temperature and precipitation using Köppen-style thresholds.
It uses no external climate data: the world is fictional and the map is rescaled, so real rainfall records don't apply.

## Usage

```text
go run ./cmd/hmz2bio [flags] <terrain.json>
```

Flags:

- `-top-lat <deg>` is the latitude of the map's top edge, north positive. The default is `27`.
- `-bottom-lat <deg>` is the latitude of the map's bottom edge. The default is `7`.
- `-wind-rays <n>` is the number of rays traced for each wind (see [Moisture and lift](#moisture-and-lift)). The default is `5`; at least 1.
- `-wind-spread <deg>` is the largest offset of a wind's rays from its bearing, in degrees. The default is `20`; at least 0.
- `-rivers <file>` is an `hmz2riv` JSON file, used only to draw rivers on the preview. Optional; its grid must match.
- `-output <file>` is the JSON file to write. Required.
- `-preview <file>` is a PNG preview to write. Optional.
- `-preview-scale <n>` sets how many raster pixels, in each direction, one preview pixel covers. The default is `8`.
- `-version` prints the version.

The command prints the time taken by each phase and the number of land hexes with each biome and surface.
A full run on the Panama terrain takes under a second.

The terrain file is read with `hmz2ter`'s own types, and the grid geometry comes from the `hmz2ele` package, rebuilt from the heightmap dimensions and apothem recorded in the terrain file.

## Units

Elevations are real meters, as in the source; the campaign map scaled distances by about 3.4, but not heights.
Temperature therefore uses real meters, which is right for a lapse rate.
Distances along the wind are **campaign** kilometers (a hex is 10 km): the air crosses the campaign world, which is the size the players see.
Orographic effects depend only on the height the air gains, which is the same in both worlds, so the gentler campaign slopes don't matter.

## Placement and wind

The map spans **27°N at the top to 7°N at the bottom**, about 20° for its 2,220 campaign km.
A hex's latitude is interpolated linearly from the top edge (y = 0) to the bottom edge (y = the raster height) at the hex center's y, and rounded to 0.01°; everything else uses the rounded latitude.

The prevailing wind follows latitude bands:

| Latitude | Wind |
| -------- | ---- |
| up to 27° | trade winds from 60° (east-north-east) |
| 27° to 33° | a linear blend of the two |
| 33° and more | westerlies from 255° (west-south-west) |

South of the equator the bearings are mirrored north–south (south-east trades, west-north-west westerlies).
With the Panama placement the whole map is in the trades: the Caribbean (east) side is windward and the Pacific (west) side is in the rain shadow of the cordillera.

## Temperature

- **Mean annual temperature** is the sea-level temperature for the latitude, from the table below, less **6.5 °C per 1,000 m** of the hex's median elevation.
- **Annual range** (warmest month minus coldest) is 1 °C + 0.33 °C per degree of latitude, split evenly about the mean. This is the only use of seasons.

| Latitude        |   0° |  10° |  15° |  20° |  25° |  30° |  35° |  40° |  45° |  50° |  55° |  60° |  70° |  80° |  90° |
| --------------- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Sea level, °C   |   27 |   27 | 26.5 |   25 | 22.5 |   20 |   17 | 14.5 |   12 |    8 |    4 |    0 |   −8 |  −16 |  −22 |

Tables are interpolated linearly in latitude, by distance from the equator, and held flat beyond their ends.

## Precipitation

Annual precipitation is:

```text
P = windward × (share + (1 − share) × moisture × (1 + 1.5 × lift / 1000))
```

- **windward** is the precipitation on a windward coast at sea level for the latitude (table below).
- **share** is the convective share: the part that falls whatever the wind does, standing in for the tropical wet season's storms, so leeward coasts aren't deserts.
- **moisture** is the share of the sea air's moisture that the wind still carries when it reaches the hex.
- **lift** is how far, in meters, the wind climbs onto the hex; windward slopes get more rain.

| Latitude        |    0° |    5° |   10° |   15° |   20° |   25° |  30° |   35° |   40° |   45° |   50° |   55° |   60° |  70° | 80° | 90° |
| --------------- | ----: | ----: | ----: | ----: | ----: | ----: | ---: | ----: | ----: | ----: | ----: | ----: | ----: | ---: | --: | --: |
| Windward, mm    | 3,100 | 3,300 | 2,700 | 2,000 | 1,400 | 1,000 |  850 | 1,000 | 1,250 | 1,400 | 1,400 | 1,250 | 1,000 |  500 | 250 | 150 |

| Latitude          |   0° |  10° |  20° |  30° |
| ----------------- | ---: | ---: | ---: | ---: |
| Convective share  | 0.50 | 0.45 | 0.30 | 0.20 |

### Moisture and lift

Each wind is traced along a **fan** of rays rather than a single line.
With `n` rays (`-wind-rays`) and a spread of `s` degrees (`-wind-spread`), ray `i` (0-based) follows the wind's bearing plus the offset:

```text
offset_i = −s + 2·s·i / (n − 1)
```

A single ray (`n = 1`) has offset 0, the wind's own bearing.
The defaults, 5 rays and 20°, give the bearing −20°, −10°, 0°, +10°, and +20°: from 40° to 80° for the 60° trades.

A single ray made the map striped: every hex downwind of the same peak or gap, along the same line, inherited the same moisture, and the biome thresholds turned those lines into straight bands of color parallel to the wind.
Averaging over a fan spreads a peak's rain shadow the way real wind does, so the bands become gradients; broad rain shadows behind whole ranges remain.

For each land hex, each wind that has weight at its latitude, and each of that wind's rays, along the ray's bearing:

1. **Trace upwind.** From the hex center, step toward the ray's bearing, 5 campaign km (48 px at apothem 48) at a time, for at most 1,000 km. At each step, find the pixel containing the point and take the hex containing that pixel's center, as `hmz2ter` assigns pixels to hexes. The point's coordinates are rounded to 6 decimals before taking `floor(x)` and `floor(y)`, so a point exactly on a pixel edge belongs to the pixel to its right or below, whatever the floating-point error. Both rules matter: a 5 km step is exactly one apothem, so on the trades' central 60° ray every other step lands exactly on a hex edge (which a pixel center never does), and every step's y is a whole number of pixels. Stop at the first point that is off the raster, in a hex not in the grid, or in salt water, cliffs, or badlands: the wind picks up moisture there, because the game world has sea beyond the border cuts and the map's edges.
2. **Build the profile**: 0 m for the sea, then the median elevation of each hex stepped through, farthest first, then the hex's own median elevation. Lakes count at their surface elevation. A step can land in the same hex as the step before, or in the hex itself; it still counts.
3. **Moisture** starts at 1 and, for each step along the profile, is multiplied by `exp(−5 / 800 − rise / 1500)`, where `rise` is the height gained since the previous point, in meters (0 when descending). So 800 km of lowland takes away 63% of the moisture, and so does a climb of 1,500 m. Air that has crossed a mountain range arrives dry: the rain shadow.
4. **Lift** is the hex's elevation minus the lowest point among the 6 profile points before it (30 km upwind), or 0 if none is lower. With fewer than 6 points before the hex, the sea's 0 m is among them.

A wind's moisture and lift are the **means** over its rays: the sums, in ray order (`i` from 0), divided by `n`.
Where the trades and westerlies are blended, each wind's means are blended with the same weights.
Moisture is rounded to 3 decimals and lift to the meter **before** computing precipitation.

## Biomes

A land hex's biome comes from its rounded climate values (see [Output](#output)), so it can be rederived from the JSON.
The first rule that applies wins:

| # | Test | Biome |
| -: | ---- | ----- |
| 1 | warmest month below 0 °C | `clear` (the surface is `glacial-ice`) |
| 2 | warmest month below 10 °C (the tree line) | `tundra` if the warmest month at **sea level** for the latitude is also below 10 °C, else `alpine` |
| 3 | precipitation below half the aridity limit | `desert` |
| 4 | precipitation below the aridity limit, mean at least 18 °C | `scrubland` |
| 5 | precipitation below the aridity limit | `steppe` |
| 6 | mean below 18 °C (montane), on `hills`, `mountains`, `plateaus`, or `volcanic-highlands`, sea-level coldest month at least 15 °C, lift at least 100 m, precipitation at least 1,000 mm | `cloud-forest` |
| 7 | mean below 18 °C, sea-level coldest month at least 18 °C (the tropics) | `temperate-forest` |
| 8 | mean at least 18 °C, precipitation at least 2,000 mm | `tropical-rainforest` |
| 9 | mean at least 18 °C, precipitation at least 1,200 mm | `tropical-dry-forest` |
| 10 | mean at least 18 °C | `savanna` |
| 11 | mean below 4 °C | `boreal-forest` |
| 12 | precipitation at least 2,000 mm | `temperate-rainforest` |
| 13 | precipitation below 1.5 × the aridity limit | `grassland` |
| 14 | otherwise | `temperate-forest` |

- The **aridity limit** is Köppen's: `20 × T + 280` mm, where `T` is the mean annual temperature in °C.
- The **sea-level** warmest and coldest months are the sea-level temperature for the hex's latitude ± half the annual range, with no lapse rate.
- **Tundra wins over alpine**: tundra is cold from latitude, alpine from height. On the Panama placement there is no tundra; it needs about 60° of latitude.
- **Cloud forest** is on the windward slopes of the tropical and subtropical mountains, where the rising air makes cloud; row 7 covers montane forest off those slopes (Central America's pine–oak forests).
- **Glacial ice** needs a warmest month below 0 °C, about 4,100 m at 26°N, so the Panama map has none; Barú reaches 3,431 m.

## Surfaces

Every land hex gets a surface:

1. **`glacial-ice`** if the warmest month is below 0 °C, on any landform. Its biome is `clear`.
2. Otherwise the hex is **`clear`** unless it can hold a wetland, which needs `flats` or `plains` and either:
   - `hmz2ter`'s `flat-surface` flag with a median elevation of **60 m** or less, which excludes the filled voids in the source (96 m and up on Panama); or
   - `flats` with the `coast` or `river` flag and a median elevation of **10 m** or less: tidal flats and floodplains.
3. A hex that can hold a wetland is:
   - **`salt-flats`** if its precipitation is below the aridity limit (coastal salt pans and inland playas);
   - **`mangroves`** if it has the `coast` flag and its coldest month is at least 15 °C;
   - **`bogs`** if its mean temperature is below 10 °C;
   - **`swamps`** (forested wetland) if its biome is a forest: `boreal-forest`, `temperate-forest`, `temperate-rainforest`, `tropical-dry-forest`, `tropical-rainforest`, or `cloud-forest`;
   - **`marshes`** otherwise.

The river flag adds no rain: 42% of land hexes have one, so a bonus would checkerboard the biomes, and a river carries water from elsewhere rather than making the hex's climate wetter.
It only marks floodplain flats as able to hold a wetland.

## Validation

After building, `hmz2bio` checks every hex against the terrain model's valid combinations (see the root README's "Terrain model" section) and exits with an error if any hex breaks them.
In particular, an unset surface or biome on land is an error, `clear` is the biome if and only if the surface is `glacial-ice`, wetlands, mangroves, and salt flats are only on flats and plains, and mangroves need the `coast` flag.
`hmz2ter`'s `flat-surface` flag isn't one of the model's flags, but it is allowed on land here as the pipeline's hint; the final hex-map emitter drops it.

## Results

On the Panama terrain (`hmz2ter` v0.2.0), 27°N to 7°N, with the default fan (5 rays, ±20°):

| Biome                 | Land hexes |  Share | v0.1.0 (1 ray) |
| --------------------- | ---------: | -----: | -------------: |
| `tropical-dry-forest` |      3,879 |  39.1% |          3,896 |
| `scrubland`           |      2,210 |  22.3% |          2,180 |
| `savanna`             |      1,511 |  15.2% |          1,357 |
| `tropical-rainforest` |      1,278 |  12.9% |          1,376 |
| `steppe`              |        495 |   5.0% |            453 |
| `desert`              |        246 |   2.5% |            329 |
| `temperate-forest`    |        159 |   1.6% |            164 |
| `cloud-forest`        |         64 |   0.6% |             92 |
| `grassland`           |         55 |   0.6% |             50 |
| `alpine`              |         20 |   0.2% |             20 |

| Surface      | Land hexes | v0.1.0 (1 ray) |
| ------------ | ---------: | -------------: |
| `clear`      |      9,704 |          9,704 |
| `mangroves`  |         63 |             63 |
| `salt-flats` |         58 |             55 |
| `swamps`     |         50 |             47 |
| `marshes`    |         42 |             48 |

No hex is `glacial-ice`, `bogs`, `tundra`, `boreal-forest`, or `temperate-rainforest`.

The south (7–15°N) is rainforest on the windward Caribbean slopes and the south-west coast, with dry forest in the Darién interior.
The middle (15–21°N) is dry forest that turns to scrub and savanna in the rain shadows, which run diagonally down and to the left from the cordillera with the east-north-east trades.
The north (23–27°N) is in the subtropical dry belt: a desert in the Chiriquí lowland behind Talamanca, steppe and montane forest on the highlands, cloud forest on their windward slopes, and alpine on the crest above about 2,600 m.

Compared with the single ray of v0.1.0, the fan removes the straight diagonal stripes of rainforest, dry forest, and savanna, most visibly in the Darién: the extremes soften (less rainforest and desert, more savanna and steppe), and cloud forest, which needs at least 100 m of lift, shrinks because lift is now averaged over rays that don't all climb the same slope.
Temperatures don't depend on the wind, so alpine is unchanged.
The v0.1.0 column is `-wind-rays 1` on the same terrain: with one ray, every hex is identical to v0.1.0's output.

Every count was cross-checked against an independent Python calculation, written from this README, with no mismatches.

## Output

```json
{
  "hmz2bio_version": "0.3.0",
  "terrain": { "file_name": "pandemokh-a48-terrain.json" },
  "hmz2ter_version": "0.2.0",
  "heightmap": { ... }, "grid": { ... }, "rivers": { ... }, "rules": { ... }, "method": { ... },
  "lakes": [ ... ], "volcanoes": [ ... ], "stats": { ... },
  "climate_rules": { "top_lat_deg": 27, "bottom_lat_deg": 7, "sea_level_temp_c": [ [0, 27], ... ], "lapse_rate_c_per_km": 6.5, ..., "wind_rays": 5, "wind_spread_deg": 20, ... },
  "climate_method": { "latitude": "...", "temperature": "...", "precipitation": "...", "biome": "...", "surface": "..." },
  "climate_stats": { "land_hexes": 9917, "biomes": { ... }, "surfaces": { ... } },
  "hexes": [
    { "col": 50, "row": 2, "landform": "mountains", "surface": "clear", "biome": "savanna", "flags": [ "impassable" ], "center": 422,
      "pixels": 7982, "valid_pixels": 7487, "land_fraction": 0.938,
      "elevation": { "min": 134, "p5": 167, "median": 341, "p95": 612, "max": 660 }, "relief_m": 445,
      "climate": { "latitude_deg": 26.77, "temperature_c": 19.4, "warmest_c": 24.3, "coldest_c": 14.5,
                   "precipitation_mm": 1085, "moisture": 0.787, "lift_m": 341 } },
    ...
  ]
}
```

Every field of the `hmz2ter` file is carried through unchanged, including each hex's; `hexes` adds `surface` and `biome` on land and a `climate` object on land hexes only.
`climate_rules` holds every setting and threshold above, including the fan's `wind_rays` and `wind_spread_deg`.

Climate values are rounded first to 6 decimals, which absorbs floating-point noise, and then half away from zero: latitude to 0.01°, temperatures to 0.1 °C, precipitation to 1 mm, moisture to 0.001, and lift to 1 m.
The mean temperature, warmest, and coldest months are each rounded from the unrounded mean and range.
Biomes and surfaces are chosen from the rounded values.

## Preview

The preview is the `hmz2ele` preview with each hex refilled: land by its surface where it has one (mangroves bright teal-green, swamps teal, marshes aqua, bogs mauve, salt flats near-white, ice pale blue), and otherwise by its biome, from deep green (tropical rainforest) through olive (tropical dry forest), gold (savanna), tan (scrubland), straw (steppe), and sand (desert), with cloud forest misty blue-green, temperate forest mid-green, and alpine gray.
Water, cliffs, and badlands are colored as in `hmz2ter`'s preview, volcano hexes magenta, and rivers dark blue along hex edges.

## License

MIT. See `LICENSE`.
