// Package horizon computes the apparent horizon profile from a DEM,
// accounting for terrain elevation and Earth curvature.
//
// The horizon profile is an array of elevation angles (degrees) for each azimuth
// step around the observer. A value of 5° at azimuth 90° means that at due east,
// the nearest visible sky is 5° above the astronomical horizon — the sun must
// climb above this to become visible.
//
// Algorithm:
//   For each azimuth (0° to 360°, step Δaz):
//     March outward along the great-circle path.
//     At each step, compute:
//       apparent_elevation = atan2(sample_elev - observer_elev - curvature_drop, distance)
//     Track the running maximum — this is the horizon elevation at this azimuth.
//
// Earth curvature: at distance d, the surface drops ~ d²/(2R) where R ≈ 6,371,000 m.
// This is factored into the apparent elevation calculation.
package horizon

import (
	"log"
	"math"
	"sync"
)

const (
	earthRadiusM = 6_371_000.0 // Earth's mean radius in meters

	// Azimuth resolution: number of samples around the full circle.
	// 360 = 1° steps. Good balance for 30m DEM data.
	defaultAzimuthSteps = 360

	// Maximum trace distance in meters. Beyond ~50 km, Earth curvature
	// drops the surface by ~200 m, so even mountain ranges are barely visible.
	maxDistanceM = 50_000.0
)

// Profile is the computed horizon profile: horizon elevation in degrees for
// each azimuth bin (evenly spaced around the full circle).
type Profile struct {
	Lat        float64   `json:"lat"`        // observer location
	Lon        float64   `json:"lon"`        // observer location
	Elevation  float64   `json:"elevation"`  // observer's ground elevation (meters)
	AzSteps    int       `json:"azSteps"`    // number of azimuth bins
	Elevations []float64 `json:"elevations"` // horizon elevation (degrees) per azimuth bin, length = AzSteps
}

// ElevationProvider is the interface for querying ground elevation.
type ElevationProvider interface {
	Elevation(lat, lon float64) float64
}

// Compute calculates the horizon profile for the given location.
// provider supplies ground elevation data (typically an srtm.Manager).
func Compute(provider ElevationProvider, lat, lon float64) *Profile {
	baseElev := provider.Elevation(lat, lon)
	noData := math.IsNaN(baseElev)
	if noData {
		baseElev = 0 // assume sea level if no data
	}

	steps := defaultAzimuthSteps
	elevs := make([]float64, steps)

	// If we have no elevation data for the observer location, we cannot
	// compute a terrain horizon. Return a flat sea-level horizon so that
	// corrected times match standard times.
	if noData {
		for i := range elevs {
			elevs[i] = 0
		}
		return &Profile{
			Lat:        lat,
			Lon:        lon,
			Elevation:  0, // no SRTM data available
			AzSteps:    steps,
			Elevations: elevs,
		}
	}

	progress := make(chan int, steps)

	azStep := 360.0 / float64(steps)

	// Each azimuth is independent — parallelize.
	var wg sync.WaitGroup
	for i := 0; i < steps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			az := float64(idx) * azStep
			elevs[idx] = traceRay(provider, lat, lon, baseElev, az)
			progress <- 1
		}(i)
	}

	// Log progress while waiting.
	go func() {
		done := 0
		for range progress {
			done++
			if done%90 == 0 || done == steps {
				log.Printf("horizon profile: %d/%d rays (%.0f%%)", done, steps,
					float64(done)/float64(steps)*100)
			}
			if done == steps {
				return
			}
		}
	}()

	wg.Wait()
	close(progress)

	return &Profile{
		Lat:        lat,
		Lon:        lon,
		Elevation:  baseElev,
		AzSteps:    steps,
		Elevations: elevs,
	}
}

// traceRay marches outward along one azimuth and returns the maximum
// apparent elevation angle (the horizon elevation at this azimuth).
func traceRay(provider ElevationProvider, lat, lon, baseElev, azimuth float64) float64 {
	maxAngle := -90.0 // start below any possible horizon

	// Adaptive step sizes: fine near the observer, coarser far away.
	// This is where most of the computational cost lives.
	dist := 0.0
	for dist < maxDistanceM {
		// Adaptive step: 30m up to 1km, then scales with distance.
		step := 30.0
		if dist > 1000 {
			step = dist * 0.05 // 5% of current distance
		}
		if step < 30 {
			step = 30
		}
		if step > 500 {
			step = 500
		}

		// Advance.
		dist += step
		if dist > maxDistanceM {
			break
		}

		// Convert distance + azimuth → lat/lon (approximate flat-earth for speed).
		// For 150 km max, the error is negligible (< 0.02°).
		plat, plon := destination(lat, lon, dist, azimuth)
		sampleElev := provider.Elevation(plat, plon)

		if math.IsNaN(sampleElev) {
			// Ocean or no data — treat as sea level with curvature drop.
			sampleElev = 0
		}

		// Earth curvature: the surface drops by d²/(2R) over distance d.
		drop := dist * dist / (2.0 * earthRadiusM)

		// Apparent elevation of this terrain point from the observer.
		apparentElev := math.Atan2(sampleElev-baseElev-drop, dist) * (180.0 / math.Pi)

		if apparentElev > maxAngle {
			maxAngle = apparentElev
		}
	}

	return maxAngle
}

// destination computes a new lat/lon given an origin, distance (meters),
// and azimuth (degrees). Uses a simple spherical Earth approximation
// accurate enough for distances up to ~200 km.
func destination(lat, lon, dist, azimuth float64) (float64, float64) {
	latRad := lat * math.Pi / 180.0
	lonRad := lon * math.Pi / 180.0
	azRad := azimuth * math.Pi / 180.0

	angularDist := dist / earthRadiusM

	newLatRad := math.Asin(
		math.Sin(latRad)*math.Cos(angularDist) +
			math.Cos(latRad)*math.Sin(angularDist)*math.Cos(azRad),
	)

	newLonRad := lonRad + math.Atan2(
		math.Sin(azRad)*math.Sin(angularDist)*math.Cos(latRad),
		math.Cos(angularDist)-math.Sin(latRad)*math.Sin(newLatRad),
	)

	return newLatRad * 180.0 / math.Pi, newLonRad * 180.0 / math.Pi
}

// HorizonElevation returns the horizon elevation angle (degrees) at the given
// azimuth from a precomputed profile.
func (p *Profile) HorizonElevation(azimuth float64) float64 {
	// Normalize azimuth to [0, 360).
	az := math.Mod(azimuth, 360)
	if az < 0 {
		az += 360
	}

	idx := int(az / 360.0 * float64(p.AzSteps))
	if idx >= p.AzSteps {
		idx = p.AzSteps - 1
	}
	return p.Elevations[idx]
}
