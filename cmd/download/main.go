// Command download fetches SRTM elevation tiles for a given location.
//
// Usage:
//
//	go run ./cmd/download -lat 47.2 -lon 11.3 -dir ./data/srtm
//
// This downloads the 3×3 neighborhood of tiles around the point, covering
// ~100+ km in all directions — enough for horizon ray tracing.
package main

import (
	"flag"
	"log"

	"github.com/philippthaler/terrain-sunset/internal/srtm"
)

func main() {
	lat := flag.Float64("lat", 0, "latitude (-90 to 90)")
	lon := flag.Float64("lon", 0, "longitude (-180 to 180)")
	dir := flag.String("dir", "./data/srtm", "directory to save tiles")
	flag.Parse()

	if *lat == 0 && *lon == 0 {
		log.Fatal("please provide -lat and -lon")
	}

	log.Printf("downloading SRTM tiles around %.4f, %.4f to %s ...", *lat, *lon, *dir)
	if err := srtm.DownloadNeighborhood(*dir, *lat, *lon); err != nil {
		log.Fatal(err)
	}
	log.Println("done! tiles saved to", *dir)
}
