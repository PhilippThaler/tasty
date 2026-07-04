package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/philippthaler/terrain-sunset/internal/api"
	"github.com/philippthaler/terrain-sunset/internal/srtm"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dataDir := flag.String("data", "./data/srtm", "SRTM .hgt data directory")
	staticDir := flag.String("web", "./web", "Frontend static files directory")
	flag.Parse()

	// Validate data directory.
	if _, err := os.Stat(*dataDir); os.IsNotExist(err) {
		log.Printf("data directory %s does not exist, creating...", *dataDir)
		if err := os.MkdirAll(*dataDir, 0o755); err != nil {
			log.Fatalf("create data dir: %v", err)
		}
	}

	// Load SRTM tiles.
	mgr := srtm.NewManager(*dataDir)
	mgr.SetAutoDownload(true)
	if err := mgr.Preload(); err != nil {
		log.Printf("warning: could not preload tiles: %v", err)
		log.Printf("you can download tiles with: go run ./cmd/download -lat <lat> -lon <lon> -dir %s", *dataDir)
	}

	// Set up routes.
	mux := http.NewServeMux()
	server := api.NewServer(mgr)
	server.RegisterRoutes(mux, *staticDir)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("terrain-sunset listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
