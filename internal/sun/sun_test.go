package sun

import (
	"math"
	"testing"
	"time"
)

func TestAtAzimuthConversion(t *testing.T) {
	// Innsbruck, solar noon on 2026-07-01.
	// At solar noon the sun is due south.
	loc := time.UTC
	noon := time.Date(2026, 7, 1, 11, 19, 20, 0, loc)
	p := At(noon, 47.27, 11.40)

	if math.Abs(p.Azimuth-180.0) > 2.0 {
		t.Errorf("solar noon azimuth = %.2f°, want ~180° (south)", p.Azimuth)
	}
	if p.Elevation < 60 || p.Elevation > 70 {
		t.Errorf("solar noon elevation = %.2f°, want ~65°", p.Elevation)
	}
}

func TestDayReturnsSunriseBeforeSunset(t *testing.T) {
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	times := Day(date, 47.27, 11.40)

	if times.Sunrise.IsZero() || times.Sunset.IsZero() {
		t.Fatal("expected non-zero sunrise/sunset")
	}
	if !times.Sunrise.Before(times.Sunset) {
		t.Errorf("sunrise %v is not before sunset %v", times.Sunrise, times.Sunset)
	}
	if !times.Sunrise.Before(times.SolarNoon) || !times.SolarNoon.Before(times.Sunset) {
		t.Errorf("solar noon %v not between sunrise %v and sunset %v",
			times.SolarNoon, times.Sunrise, times.Sunset)
	}
}

func TestDayPolarNight(t *testing.T) {
	// Tromsø, December 21 — polar night.
	date := time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC)
	times := Day(date, 69.65, 18.96)

	if !times.Sunrise.IsZero() || !times.Sunset.IsZero() {
		t.Errorf("expected polar night (no sunrise/sunset), got rise=%v set=%v",
			times.Sunrise, times.Sunset)
	}
}
