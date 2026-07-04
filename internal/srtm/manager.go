package srtm

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// Manager loads and caches SRTM tiles from a data directory.
// It automatically loads neighboring tiles as needed during horizon ray tracing.
type Manager struct {
	mu           sync.RWMutex
	dir          string           // directory containing .hgt files
	tiles        map[string]*Tile // cache: tile name → loaded tile
	autoDownload bool             // download missing tiles on demand

	dlMu        sync.Mutex
	downloading map[string]*sync.Mutex // per-neighborhood download locks
}

// NewManager creates a tile manager that reads .hgt files from dir.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:         dir,
		tiles:       make(map[string]*Tile),
		downloading: make(map[string]*sync.Mutex),
	}
}

// SetAutoDownload enables or disables on-demand downloading of missing tiles.
func (m *Manager) SetAutoDownload(enabled bool) {
	m.autoDownload = enabled
}

// Elevation returns the elevation at (lat, lon). Loads the appropriate tile
// from disk if not already cached. Returns NaN for ocean/no-data areas.
func (m *Manager) Elevation(lat, lon float64) float64 {
	tile, err := m.getTile(lat, lon)
	if err != nil || tile == nil {
		return math.NaN()
	}
	return tile.Elevation(lat, lon)
}

// Preload loads all tiles in the data directory into memory.
// Call this at startup for best performance.
func (m *Manager) Preload() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("read data dir %s: %w", m.dir, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".hgt" {
			continue
		}
		path := filepath.Join(m.dir, e.Name())
		tile, err := LoadTile(path)
		if err != nil {
			// Log but continue — one bad tile shouldn't break everything.
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			continue
		}
		m.tiles[tile.Name] = tile
		fmt.Printf("srtm: loaded %s (%d×%d)\n", tile.Name, tile.Size, tile.Size)
	}

	return nil
}

// getTile returns the tile covering (lat, lon), loading it from disk if needed.
// If auto-download is enabled and the tile is missing, it downloads the 3×3
// neighborhood first.
func (m *Manager) getTile(lat, lon float64) (*Tile, error) {
	name := tileName(lat, lon)

	// Fast path: check cache with read lock.
	m.mu.RLock()
	tile, ok := m.tiles[name]
	m.mu.RUnlock()
	if ok {
		return tile, nil
	}

	// Try to download missing tiles if enabled.
	if m.autoDownload {
		if err := m.downloadNeighborhood(lat, lon); err != nil {
			// Log but continue — maybe the tile exists locally or download failed.
			fmt.Fprintf(os.Stderr, "srtm: auto-download failed for %s: %v\n", name, err)
		}
	}

	// Slow path: load from disk with write lock.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check: another goroutine might have loaded it while we waited.
	if tile, ok = m.tiles[name]; ok {
		return tile, nil
	}

	path := filepath.Join(m.dir, name+".hgt")
	tile, err := LoadTile(path)
	if err != nil {
		return nil, err
	}
	m.tiles[name] = tile
	return tile, nil
}

// downloadNeighborhood downloads the 3×3 tile block around (lat, lon), using a
// per-neighborhood lock so concurrent requests for the same area only download
// once.
func (m *Manager) downloadNeighborhood(lat, lon float64) error {
	key := tileName(lat, lon)

	m.dlMu.Lock()
	mu, ok := m.downloading[key]
	if !ok {
		mu = &sync.Mutex{}
		m.downloading[key] = mu
	}
	m.dlMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	return DownloadNeighborhood(m.dir, lat, lon)
}

// tileName returns the SRTM tile name for a coordinate.
// e.g. (47.2, 11.3) → "N47E011"
func tileName(lat, lon float64) string {
	ns := 'N'
	if lat < 0 {
		ns = 'S'
	}
	ew := 'E'
	if lon < 0 {
		ew = 'W'
	}
	return fmt.Sprintf("%c%02d%c%03d", ns, int(math.Floor(math.Abs(lat))), ew, int(math.Floor(math.Abs(lon))))
}
