package srtm

import (
	"math"
	"os"
	"testing"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		path        string
		wantName    string
		wantLat     float64
		wantLon     float64
		wantErr     bool
	}{
		{"N47E011.hgt", "N47E011", 47, 11, false},
		{"S33W070.hgt", "S33W070", -33, -70, false},
		{"/data/srtm/N00E000.hgt", "N00E000", 0, 0, false},
		{"badname.hgt", "", 0, 0, true},
		{"N47E01.hgt", "", 0, 0, true}, // too short
	}

	for _, tt := range tests {
		name, lat, lon, err := parseFilename(tt.path)
		if tt.wantErr && err == nil {
			t.Errorf("parseFilename(%q): expected error, got nil", tt.path)
			continue
		}
		if !tt.wantErr && err != nil {
			t.Errorf("parseFilename(%q): unexpected error: %v", tt.path, err)
			continue
		}
		if err != nil {
			continue
		}
		if name != tt.wantName {
			t.Errorf("parseFilename(%q).name = %q, want %q", tt.path, name, tt.wantName)
		}
		if lat != tt.wantLat {
			t.Errorf("parseFilename(%q).lat = %v, want %v", tt.path, lat, tt.wantLat)
		}
		if lon != tt.wantLon {
			t.Errorf("parseFilename(%q).lon = %v, want %v", tt.path, lon, tt.wantLon)
		}
	}
}

func TestTileName(t *testing.T) {
	tests := []struct {
		lat, lon float64
		want     string
	}{
		{47.2, 11.3, "N47E011"},
		{-33.4, -70.6, "S33W070"},
		{0.0, 0.0, "N00E000"},
		{-0.5, -0.5, "S00W000"},
		{89.9, 179.9, "N89E179"},
	}

	for _, tt := range tests {
		got := tileName(tt.lat, tt.lon)
		if got != tt.want {
			t.Errorf("tileName(%v, %v) = %q, want %q", tt.lat, tt.lon, got, tt.want)
		}
	}
}

func TestGenerateTestHGT(t *testing.T) {
	// Write a tiny synthetic HGT file (3x3 grid) and verify we can read it.
	// Valid HGT: file size = 2 * N² bytes, so 2*9=18 bytes for a 3×3 grid.
	size := 3
	data := make([]byte, 2*size*size)

	// Fill with known values: row-major, elevations 100, 200, 300, ...
	for i := 0; i < size*size; i++ {
		val := int16((i + 1) * 100)
		data[i*2] = byte(val >> 8)   // big-endian high byte
		data[i*2+1] = byte(val & 0xFF) // low byte
	}

	tmp := t.TempDir()
	path := tmp + "/N00E000.hgt"
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	tile, err := LoadTile(path)
	if err != nil {
		t.Fatal(err)
	}

	if tile.Name != "N00E000" {
		t.Errorf("name = %q, want N00E000", tile.Name)
	}
	if tile.Size != size {
		t.Errorf("size = %d, want %d", tile.Size, size)
	}

	// Sample center of tile: lat=0.5, lon=0.5 → row=1, col=1 (since grid is 0-2)
	// row = (1 - (0.5 - 0)) * 2 = 1.0 → idx 1
	// col = (0.5 - 0) * 2 = 1.0 → idx 1
	// index = 1*3 + 1 = 4 → value = (4+1)*100 = 500
	elev := tile.Elevation(0.5, 0.5)
	if elev != 500 {
		t.Errorf("elevation at center = %v, want 500", elev)
	}

	// South-west corner: lat=0.0, lon=0.0 → row=2, col=0 → index = 2*3+0 = 6 → 700
	elev = tile.Elevation(0.0, 0.0)
	if elev != 700 {
		t.Errorf("elevation at SW corner = %v, want 700", elev)
	}

	// North-east corner: lat=1.0, lon=1.0 → row=0, col=2 → index = 0*3+2 = 2 → 300
	elev = tile.Elevation(1.0, 1.0)
	if elev != 300 {
		t.Errorf("elevation at NE corner = %v, want 300", elev)
	}

	// Outside tile
	elev = tile.Elevation(2.0, 2.0)
	if !math.IsNaN(elev) {
		t.Errorf("elevation outside tile = %v, want NaN", elev)
	}
}
