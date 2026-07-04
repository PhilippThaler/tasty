package horizon

import (
	"math"
	"testing"
	"time"

	"github.com/philippthaler/terrain-sunset/internal/sun"
)

func TestCorrectDayFlatHorizon(t *testing.T) {
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	profile := Compute(constantElevProvider{elev: 0}, 47.27, 11.40)
	got := CorrectDay(date, 47.27, 11.40, profile)

	if got.AlwaysUp || got.AlwaysDown {
		t.Errorf("flat horizon should not be always up/down")
	}

	std := sun.Day(date, 47.27, 11.40)
	if got.Sunrise.IsZero() || got.Sunset.IsZero() {
		t.Fatal("expected non-zero corrected times")
	}

	// With flat horizon + refraction, corrected times should be within a few
	// minutes of standard times.
	if math.Abs(got.Sunrise.Sub(std.Sunrise).Minutes()) > 2 {
		t.Errorf("sunrise diff = %.2f min, want < 2 min", got.Sunrise.Sub(std.Sunrise).Minutes())
	}
	if math.Abs(got.Sunset.Sub(std.Sunset).Minutes()) > 2 {
		t.Errorf("sunset diff = %.2f min, want < 2 min", got.Sunset.Sub(std.Sunset).Minutes())
	}
}

func TestCorrectDayHighHorizonDelaysSunrise(t *testing.T) {
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	std := sun.Day(date, 47.27, 11.40)

	// Artificially high eastern horizon (mountains to the east).
	profile := &Profile{
		Lat:        47.27,
		Lon:        11.40,
		Elevation:  0,
		AzSteps:    360,
		Elevations: make([]float64, 360),
	}
	for i := range profile.Elevations {
		az := float64(i)
		if az >= 45 && az <= 135 {
			profile.Elevations[i] = 10.0 // high eastern horizon
		}
	}

	got := CorrectDay(date, 47.27, 11.40, profile)
	if got.Sunrise.IsZero() {
		t.Fatal("expected non-zero sunrise")
	}
	if !got.Sunrise.After(std.Sunrise) {
		t.Errorf("high eastern horizon should delay sunrise; got %v, standard %v", got.Sunrise, std.Sunrise)
	}
}

func TestCorrectDayPolarNight(t *testing.T) {
	// Tromsø, December 21.
	date := time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC)
	profile := Compute(constantElevProvider{elev: 0}, 69.65, 18.96)
	got := CorrectDay(date, 69.65, 18.96, profile)

	if !got.AlwaysDown {
		t.Errorf("expected AlwaysDown=true for polar night, got %v", got.AlwaysDown)
	}
	if got.AlwaysUp {
		t.Errorf("expected AlwaysUp=false for polar night, got %v", got.AlwaysUp)
	}
}

func TestCorrectDayPolarDay(t *testing.T) {
	// Svalbard, June 21.
	date := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	profile := Compute(constantElevProvider{elev: 0}, 78.22, 15.65)
	got := CorrectDay(date, 78.22, 15.65, profile)

	if !got.AlwaysUp {
		t.Errorf("expected AlwaysUp=true for polar day, got %v", got.AlwaysUp)
	}
	if got.AlwaysDown {
		t.Errorf("expected AlwaysDown=false for polar day, got %v", got.AlwaysDown)
	}
}

func TestAtmosphericRefraction(t *testing.T) {
	// Near horizon, refraction is about 0.5°.
	r := atmosphericRefraction(0)
	if math.Abs(r-0.5) > 0.1 {
		t.Errorf("refraction at 0° = %.3f°, want ~0.5°", r)
	}
	// At high elevations, refraction is tiny.
	r = atmosphericRefraction(45)
	if r > 0.05 {
		t.Errorf("refraction at 45° = %.3f°, want < 0.05°", r)
	}
	// Below -5° should return 0.
	r = atmosphericRefraction(-10)
	if r != 0 {
		t.Errorf("refraction at -10° = %.3f°, want 0", r)
	}
}

func TestSunPathNoPanics(t *testing.T) {
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	profile := Compute(constantElevProvider{elev: 0}, 47.27, 11.40)
	points := SunPath(date, 47.27, 11.40, profile)
	if len(points) == 0 {
		t.Fatal("expected non-empty sun path")
	}
	for _, p := range points {
		if p.Azimuth < 0 || p.Azimuth >= 360 {
			t.Errorf("azimuth out of range: %.2f", p.Azimuth)
		}
	}
}

func TestSunPathSummerNorthernHemisphere(t *testing.T) {
	// In Northern Hemisphere summer the sun rises in the NE and sets in the NW,
	// passing south at noon.
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	profile := Compute(constantElevProvider{elev: 0}, 46.45, -10.09)
	points := SunPath(date, 46.45, -10.09, profile)

	minAz, maxAz := 360.0, 0.0
	maxElev := -90.0
	for _, p := range points {
		if p.Azimuth < minAz {
			minAz = p.Azimuth
		}
		if p.Azimuth > maxAz {
			maxAz = p.Azimuth
		}
		if p.Elevation > maxElev {
			maxElev = p.Elevation
		}
	}

	if minAz > 60 {
		t.Errorf("minimum azimuth = %.1f°, expected sunrise azimuth < 60° (NE)", minAz)
	}
	if maxAz < 300 {
		t.Errorf("maximum azimuth = %.1f°, expected sunset azimuth > 300° (NW)", maxAz)
	}
	if maxElev < 60 {
		t.Errorf("maximum elevation = %.1f°, expected solar noon elevation > 60°", maxElev)
	}
}
