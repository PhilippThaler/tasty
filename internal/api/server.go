// Package api provides the HTTP API for terrain-corrected sun calculations.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/philippthaler/terrain-sunset/internal/horizon"
	"github.com/philippthaler/terrain-sunset/internal/srtm"
)

// Server handles API requests.
type Server struct {
	srtm *srtm.Manager
	// cache stores computed horizon profiles keyed by "lat,lon".
	cache map[string]*horizon.Profile
}

// NewServer creates a new API server backed by the given SRTM manager.
func NewServer(srtmMgr *srtm.Manager) *Server {
	return &Server{
		srtm:  srtmMgr,
		cache: make(map[string]*horizon.Profile),
	}
}

// RegisterRoutes adds API routes to the given mux, plus static file serving
// for the frontend from the given directory.
func (s *Server) RegisterRoutes(mux *http.ServeMux, staticDir string) {
	mux.HandleFunc("GET /api/horizon", s.handleHorizon)
	mux.HandleFunc("GET /api/times", s.handleTimes)
	mux.HandleFunc("GET /api/sunpath", s.handleSunPath)
	mux.HandleFunc("GET /api/elevation", s.handleElevation)

	// Serve frontend static files.
	if staticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(staticDir)))
	}
}

// --- Handlers ---

// GET /api/horizon?lat=47.2&lon=11.3
func (s *Server) handleHorizon(w http.ResponseWriter, r *http.Request) {
	lat, lon, err := parseLatLon(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile := s.getOrComputeProfile(lat, lon)
	writeJSON(w, profile)
}

// GET /api/times?lat=47.2&lon=11.3&date=2026-07-01
func (s *Server) handleTimes(w http.ResponseWriter, r *http.Request) {
	lat, lon, err := parseLatLon(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	date := parseDate(r)
	profile := s.getOrComputeProfile(lat, lon)
	corrected := horizon.CorrectDay(date, lat, lon, profile)

	// Build response with human-readable times.
	type response struct {
		Lat               float64 `json:"lat"`
		Lon               float64 `json:"lon"`
		Date              string  `json:"date"`
		Elevation         float64 `json:"elevation"`
		StandardSunrise   string  `json:"standardSunrise"`
		StandardSunset    string  `json:"standardSunset"`
		SolarNoon         string  `json:"solarNoon"`
		CorrectedSunrise  string  `json:"correctedSunrise"`
		CorrectedSunset   string  `json:"correctedSunset"`
		SunriseDelayMin   float64 `json:"sunriseDelayMinutes"`
		SunsetDelayMin    float64 `json:"sunsetDelayMinutes"`
		AlwaysUp          bool    `json:"alwaysUp"`
		AlwaysDown        bool    `json:"alwaysDown"`
	}

	resp := response{
		Lat:              lat,
		Lon:              lon,
		Date:             date.Format("2006-01-02"),
		Elevation:        profile.Elevation,
		StandardSunrise:  fmtTime(corrected.Standard.Sunrise),
		StandardSunset:   fmtTime(corrected.Standard.Sunset),
		SolarNoon:        fmtTime(corrected.Standard.SolarNoon),
		CorrectedSunrise: fmtTime(corrected.Sunrise),
		CorrectedSunset:  fmtTime(corrected.Sunset),
		SunriseDelayMin:  corrected.SunriseDelay.Minutes(),
		SunsetDelayMin:   corrected.SunsetDelay.Minutes(),
		AlwaysUp:         corrected.AlwaysUp,
		AlwaysDown:       corrected.AlwaysDown,
	}

	writeJSON(w, resp)
}

// GET /api/sunpath?lat=47.2&lon=11.3&date=2026-07-01
func (s *Server) handleSunPath(w http.ResponseWriter, r *http.Request) {
	lat, lon, err := parseLatLon(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	date := parseDate(r)
	profile := s.getOrComputeProfile(lat, lon)
	points := horizon.SunPath(date, lat, lon, profile)

	writeJSON(w, map[string]any{
		"lat":    lat,
		"lon":    lon,
		"date":   date.Format("2006-01-02"),
		"points": points,
	})
}

// GET /api/elevation?lat=47.2&lon=11.3
func (s *Server) handleElevation(w http.ResponseWriter, r *http.Request) {
	lat, lon, err := parseLatLon(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	elev := s.srtm.Elevation(lat, lon)
	writeJSON(w, map[string]float64{
		"lat":       lat,
		"lon":       lon,
		"elevation": elev,
	})
}

// --- Helpers ---

func (s *Server) getOrComputeProfile(lat, lon float64) *horizon.Profile {
	key := fmt.Sprintf("%.4f,%.4f", lat, lon)
	if p, ok := s.cache[key]; ok {
		return p
	}
	log.Printf("computing horizon profile for %.4f, %.4f ...", lat, lon)
	p := horizon.Compute(s.srtm, lat, lon)
	s.cache[key] = p
	return p
}

func parseLatLon(r *http.Request) (float64, float64, error) {
	lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'lat' parameter: %w", err)
	}
	lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'lon' parameter: %w", err)
	}
	if lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("latitude out of range: %.2f", lat)
	}
	if lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("longitude out of range: %.2f", lon)
	}
	return lat, lon, nil
}

func parseDate(r *http.Request) time.Time {
	dateStr := r.URL.Query().Get("date")
	tzStr := r.URL.Query().Get("tz")

	loc := time.UTC
	if tzStr != "" {
		if l, err := time.LoadLocation(tzStr); err == nil {
			loc = l
		}
	}

	if dateStr == "" {
		return time.Now().In(loc)
	}

	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Now().In(loc)
	}
	return t
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error encoding JSON: %v", err)
	}
}
