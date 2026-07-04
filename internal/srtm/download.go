package srtm

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// ESA SRTM mirror — free, no auth, global SRTM GL1 (30m) data.
	// File pattern: N47E011.SRTMGL1.hgt.zip
	srtm1BaseURL = "https://step.esa.int/auxdata/dem/SRTMGL1/"
	srtm3BaseURL = "https://step.esa.int/auxdata/dem/SRTMGL1/"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// DownloadTile fetches an SRTM1 (30m) tile from a public mirror and saves it
// as a raw .hgt file in the given directory. The .hgt.zip is downloaded,
// unzipped, and the .hgt extracted.
//
// Returns the path to the saved .hgt file.
func DownloadTile(dir string, lat, lon float64) (string, error) {
	name := tileName(lat, lon)
	zipURL := srtm1BaseURL + name + ".SRTMGL1.hgt.zip"
	targetPath := filepath.Join(dir, name+".hgt")

	// Skip if already downloaded.
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, nil
	}

	fmt.Printf("srtm: downloading %s …\n", zipURL)

	resp, err := httpClient.Get(zipURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", zipURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", zipURL, resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// Unzip in memory.
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Ext(f.Name) != ".hgt" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		defer rc.Close()

		hgtBytes, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("read zip entry: %w", err)
		}

		if err := os.WriteFile(targetPath, hgtBytes, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", targetPath, err)
		}

		fmt.Printf("srtm: saved %s (%d bytes)\n", targetPath, len(hgtBytes))
		return targetPath, nil
	}

	return "", fmt.Errorf("no .hgt file found in zip %s", zipURL)
}

// DownloadNeighborhood downloads the 3×3 block of tiles surrounding (lat, lon).
// This ensures we have all tiles needed for horizon ray tracing up to ~100 km.
// Tiles are downloaded concurrently.
func DownloadNeighborhood(dir string, lat, lon float64) error {
	var wg sync.WaitGroup
	for dlat := -1; dlat <= 1; dlat++ {
		for dlon := -1; dlon <= 1; dlon++ {
			wg.Add(1)
			go func(dla, dlo int) {
				defer wg.Done()
				if _, err := DownloadTile(dir, lat+float64(dla), lon+float64(dlo)); err != nil {
					// Don't fail on missing tiles (oceans etc.)
					fmt.Fprintf(os.Stderr, "srtm: skip tile at d(%d,%d): %v\n", dla, dlo, err)
				}
			}(dlat, dlon)
		}
	}
	wg.Wait()
	return nil
}
