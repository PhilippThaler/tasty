package horizon

import (
	"math"
	"time"

	"github.com/philippthaler/terrain-sunset/internal/sun"
)

const (
	sunAngularRadius = 0.27 // degrees, apparent radius of the solar disk
	searchWindow     = 12 * time.Hour // wide enough for extreme terrain (e.g. deep valleys)
)

// atmosphericRefraction returns the approximate refraction in degrees for a
// geometric elevation. Near the horizon this lifts the apparent sun by ~0.5°.
// Uses Saemundsson's formula. Valid for elevations above about -5°.
func atmosphericRefraction(elevDeg float64) float64 {
	if elevDeg < -5 {
		return 0
	}
	return 0.0167 / math.Tan((elevDeg+10.3/(elevDeg+5.11))*math.Pi/180.0)
}

// CorrectedTimes holds terrain-corrected sun event times.
type CorrectedTimes struct {
	// Standard times (sea-level, no terrain).
	Standard sun.TransitTimes

	// Terrain-corrected times. These account for mountains, valleys, etc.
	// May be zero if the sun never clears the terrain that day.
	Sunrise     time.Time // when sun's upper limb first clears terrain
	Sunset      time.Time // when sun's upper limb last clears terrain

	// Difference from standard times. Positive = delayed, negative = early.
	SunriseDelay time.Duration
	SunsetDelay  time.Duration

	// Whether the sun is always above or below the terrain horizon all day.
	AlwaysUp   bool // e.g., polar day in a flat area
	AlwaysDown bool // e.g., polar night or deep valley
}

// CorrectDay computes terrain-corrected sunrise and sunset times for a given day.
// The profile must already be computed for the observer location.
func CorrectDay(date time.Time, lat, lon float64, profile *Profile) CorrectedTimes {
	std := sun.Day(date, lat, lon)
	result := CorrectedTimes{
		Standard: std,
	}

	// Handle polar day/night (standard times may not exist at high latitudes).
	if std.Sunrise.IsZero() || std.Sunset.IsZero() {
		noonPos := sun.At(std.SolarNoon, lat, lon)
		result.AlwaysUp = noonPos.Elevation > 0
		result.AlwaysDown = noonPos.Elevation <= 0
		return result
	}

	// Search a wide window around the standard time in BOTH directions.
	// In a valley, sunrise is delayed (mountains block low sun).
	// On a mountain top, sunrise can be earlier (you see over lower terrain).
	// We search the full window and pick the transition closest to standard.
	result.Sunrise = findFirstTransition(std.Sunrise, searchWindow, lat, lon, profile, true)
	if !result.Sunrise.IsZero() {
		result.SunriseDelay = result.Sunrise.Sub(std.Sunrise)
	}

	result.Sunset = findFirstTransition(std.Sunset, searchWindow, lat, lon, profile, false)
	if !result.Sunset.IsZero() {
		result.SunsetDelay = result.Sunset.Sub(std.Sunset)
	}

	// If neither transition was found, the terrain permanently blocks or exposes the sun.
	if result.Sunrise.IsZero() && result.Sunset.IsZero() {
		noonPos := sun.At(std.SolarNoon, lat, lon)
		noonHorizon := profile.HorizonElevation(noonPos.Azimuth)
		if noonPos.Elevation > noonHorizon {
			result.AlwaysUp = true
		} else {
			result.AlwaysDown = true
		}
	}

	return result
}

// findFirstTransition searches for the sun crossing the terrain horizon near refTime.
// rising=true: find first moment sun goes from below→above terrain (sunrise).
// rising=false: find last moment sun goes from above→below terrain (sunset).
//
// Searches within ±window of refTime at 10-second resolution.
func findFirstTransition(refTime time.Time, window time.Duration,
	lat, lon float64, profile *Profile, rising bool,
) time.Time {
	start := refTime.Add(-window)
	end := refTime.Add(window)
	step := 10 * time.Second

	var bestTime time.Time
	bestDist := time.Duration(1<<63 - 1) // max duration

	prevAbove := false
	first := true

	for t := start; t.Before(end) || t.Equal(end); t = t.Add(step) {
		sp := sun.At(t, lat, lon)
		horizonElev := profile.HorizonElevation(sp.Azimuth)

		// Apparent elevation includes the solar disk (upper limb) and atmospheric
		// refraction. This matches how standard sunrise/sunset times are defined.
		apparentElev := sp.Elevation + sunAngularRadius + atmosphericRefraction(sp.Elevation)

		above := apparentElev > horizonElev

		if !first {
			if rising && !prevAbove && above {
				// Sun just cleared terrain horizon → candidate sunrise.
				dist := absDuration(t.Sub(refTime))
				if dist < bestDist {
					bestDist = dist
					bestTime = t
				}
			}
			if !rising && prevAbove && !above {
				// Sun just dipped below terrain → candidate sunset.
				dist := absDuration(t.Sub(refTime))
				if dist < bestDist {
					bestDist = dist
					bestTime = t
				}
			}
		}

		prevAbove = above
		first = false
	}

	return bestTime
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// SunPathPoint represents one sample of the sun's path across the sky.
type SunPathPoint struct {
	Time      time.Time `json:"time"`
	Azimuth   float64   `json:"azimuth"`
	Elevation float64   `json:"elevation"`
	// TerrainHorizon is the horizon elevation at this azimuth (degrees).
	TerrainHorizon float64 `json:"terrainHorizon"`
	// Visible is true if the sun's upper limb is above the terrain horizon.
	Visible bool `json:"visible"`
}

// SunPath computes the sun's path across the sky for a single day,
// annotated with terrain horizon intersections.
// Returns points at 2-minute intervals from dawn to dusk.
func SunPath(date time.Time, lat, lon float64, profile *Profile) []SunPathPoint {
	std := sun.Day(date, lat, lon)

	// Use civil dawn/dusk as range, extended slightly.
	start := std.DawnCivil.Add(-30 * time.Minute)
	end := std.DuskCivil.Add(30 * time.Minute)
	interval := 2 * time.Minute

	// Estimate number of points.
	dur := end.Sub(start)
	n := int(dur / interval)
	if n <= 0 || n > 2000 {
		n = 500 // safe default
	}
	points := make([]SunPathPoint, 0, n)

	for t := start; t.Before(end) || t.Equal(end); t = t.Add(interval) {
		sp := sun.At(t, lat, lon)
		horizonElev := profile.HorizonElevation(sp.Azimuth)

		apparentElev := sp.Elevation + atmosphericRefraction(sp.Elevation)
		visible := apparentElev > horizonElev

		points = append(points, SunPathPoint{
			Time:           t,
			Azimuth:        round(sp.Azimuth, 2),
			Elevation:      round(sp.Elevation, 2),
			TerrainHorizon: round(horizonElev, 2),
			Visible:        visible,
		})
	}

	return points
}

func round(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
