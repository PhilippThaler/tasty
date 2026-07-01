// Package srtm reads Shuttle Radar Topography Mission (SRTM) elevation data.
//
// SRTM .hgt files are raw binary grids of 16-bit signed integers (big-endian).
// Each file covers a 1°×1° tile. Files come in two resolutions:
//
//	SRTM1 (1 arcsecond ≈ 30m):  3601×3601 grid, ~25.9 MB
//	SRTM3 (3 arcsecond ≈ 90m):  1201×1201 grid, ~2.9 MB
//
// Values are meters above sea level. -32768 (0x8000) marks void/no-data cells.
// Rows go from north (row 0) to south (row N-1).
//
// Tile naming: N47E011.hgt means latitude 47°N, longitude 11°E.
package srtm

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// Void elevation marker in SRTM data.
const voidElevation = -32768

// Tile represents one SRTM 1°×1° tile loaded into memory.
type Tile struct {
	Name       string  // e.g. "N47E011"
	Lat        float64 // integer latitude of the tile's south edge (e.g. 47.0)
	Lon        float64 // integer longitude of the tile's west edge (e.g. 11.0)
	Size       int     // grid dimension (3601 for SRTM1, 1201 for SRTM3)
	data       []int16 // row-major, north→south (row 0 = north edge)
}

// LoadTile reads a .hgt file from disk. It auto-detects resolution from file size.
func LoadTile(path string) (*Tile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tile %s: %w", path, err)
	}

	// Each cell is 2 bytes (int16). File size must be 2 * N².
	half := len(data) / 2
	size := int(math.Sqrt(float64(half)))
	if size*size != half {
		return nil, fmt.Errorf("invalid .hgt file %s: size %d is not 2×N²", path, len(data))
	}

	// Parse name from filename: e.g. "N47E011.hgt" → name="N47E011", lat=47, lon=11
	name, lat, lon, err := parseFilename(path)
	if err != nil {
		return nil, err
	}

	// Decode big-endian int16 grid.
	grid := make([]int16, half)
	for i := range grid {
		grid[i] = int16(binary.BigEndian.Uint16(data[i*2 : i*2+2]))
	}

	return &Tile{
		Name: name,
		Lat:  lat,
		Lon:  lon,
		Size: size,
		data: grid,
	}, nil
}

// Elevation returns the elevation in meters at the given (lat, lon) within this tile.
// Uses bilinear interpolation between the four nearest grid cells.
// Returns NaN if the point is outside the tile or falls on void data.
func (t *Tile) Elevation(lat, lon float64) float64 {
	if lat < t.Lat || lat > t.Lat+1 || lon < t.Lon || lon > t.Lon+1 {
		return math.NaN()
	}

	// Fractional position within tile (0..1)
	// Row 0 is the north edge → frow = 1 - (lat - t.Lat)
	fcol := (lon - t.Lon) * float64(t.Size-1)
	frow := (1.0 - (lat - t.Lat)) * float64(t.Size-1)

	// Integer grid indices
	col := int(fcol)
	row := int(frow)

	// Fractional part for interpolation
	dc := fcol - float64(col)
	dr := frow - float64(row)

	// Get four neighbors, handling edges
	v00 := t.sample(row, col)
	v01 := t.sample(row, min(col+1, t.Size-1))
	v10 := t.sample(min(row+1, t.Size-1), col)
	v11 := t.sample(min(row+1, t.Size-1), min(col+1, t.Size-1))

	// If any neighbor is void, return NaN
	if math.IsNaN(v00) || math.IsNaN(v01) || math.IsNaN(v10) || math.IsNaN(v11) {
		return math.NaN()
	}

	// Bilinear interpolation
	top := v00*(1-dc) + v01*dc
	bot := v10*(1-dc) + v11*dc
	return top*(1-dr) + bot*dr
}

// sample returns the elevation at grid (row, col) or NaN for void data.
func (t *Tile) sample(row, col int) float64 {
	v := t.data[row*t.Size+col]
	if v == voidElevation {
		return math.NaN()
	}
	return float64(v)
}

// parseFilename extracts tile name, latitude, and longitude from a .hgt file path.
// Expects format like ".../N47E011.hgt".
func parseFilename(path string) (name string, lat, lon float64, err error) {
	// Extract basename: strip directories and extension.
	base := path
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' {
			base = base[i+1:]
			break
		}
	}
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '.' {
			base = base[:i]
			break
		}
	}

	// Parse: first char is N/S, then two digits lat, then E/W, then three digits lon.
	if len(base) != 7 {
		return "", 0, 0, fmt.Errorf("invalid tile name %q: expected format N47E011", base)
	}

	name = base

	// Latitude
	latSign := 1.0
	if base[0] == 'S' {
		latSign = -1.0
	} else if base[0] != 'N' {
		return "", 0, 0, fmt.Errorf("invalid tile name %q: first char must be N or S", base)
	}
	fmt.Sscanf(base[1:3], "%f", &lat)
	lat = latSign * lat

	// Longitude
	lonSign := 1.0
	if base[3] == 'W' {
		lonSign = -1.0
	} else if base[3] != 'E' {
		return "", 0, 0, fmt.Errorf("invalid tile name %q: fourth char must be E or W", base)
	}
	fmt.Sscanf(base[4:7], "%f", &lon)
	lon = lonSign * lon

	return name, lat, lon, nil
}
