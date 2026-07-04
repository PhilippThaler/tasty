package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/philippthaler/terrain-sunset/internal/srtm"
)

func newTestServer() *Server {
	return NewServer(srtm.NewManager("./testdata-empty"))
}

func TestHandleElevation(t *testing.T) {
	// Use a real SRTM tile from the existing testdata if available, otherwise
	// the manager returns NaN and we verify null JSON.
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux, "")

	req := httptest.NewRequest("GET", "/api/elevation?lat=30&lon=-39", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	if resp["elevation"] != nil {
		t.Errorf("elevation = %v, want null for missing data", resp["elevation"])
	}
}

func TestHandleTimesInvalidLat(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux, "")

	req := httptest.NewRequest("GET", "/api/times?lat=abc&lon=11", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandleTimesInvalidDate(t *testing.T) {
	srv := newTestServer()
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux, "")

	req := httptest.NewRequest("GET", "/api/times?lat=47&lon=11&date=bogus", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestParseLatLon(t *testing.T) {
	cases := []struct {
		query string
		ok    bool
	}{
		{"lat=47.27&lon=11.40", true},
		{"lat=abc&lon=11.40", false},
		{"lat=47.27&lon=def", false},
		{"lat=91&lon=0", false},
		{"lat=0&lon=181", false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/?"+tc.query, nil)
		_, _, err := parseLatLon(req)
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error: %v", tc.query, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error", tc.query)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		query string
		ok    bool
	}{
		{"date=2026-07-01", true},
		{"date=bogus", false},
		{"", true}, // missing date defaults to now
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/?"+tc.query, nil)
		_, err := parseDate(req)
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error: %v", tc.query, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error", tc.query)
		}
	}
}

// floatOrNil helper used by tests.
func floatOrNil(v any) (float64, bool) {
	if v == nil {
		return math.NaN(), false
	}
	f, ok := v.(float64)
	return f, ok
}
