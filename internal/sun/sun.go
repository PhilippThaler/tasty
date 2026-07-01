// Package sun provides sun position calculations wrapping github.com/kixorz/suncalc.
//
// All angles are returned in degrees. Azimuth: 0°=North, 90°=East, 180°=South, 270°=West.
// Elevation: 0°=horizon, 90°=zenith, negative=below horizon.
package sun

import (
	"math"
	"time"

	"github.com/kixorz/suncalc"
)

// Position holds the sun's apparent position at a given time and location.
type Position struct {
	Azimuth   float64 // degrees, 0=North, clockwise
	Elevation float64 // degrees, 0=horizon, positive=above
}

// At returns the sun position at the given time.
func At(t time.Time, lat, lon float64) Position {
	p := suncalc.GetPosition(t, lat, lon)
	// suncalc returns azimuth where 0°=South, positive=West.
	// Convert to 0°=North, 90°=East (clockwise).
	az := radToDeg(p.Azimuth) + 180
	if az >= 360 {
		az -= 360
	}
	return Position{
		Azimuth:   az,
		Elevation: radToDeg(p.Altitude),
	}
}

// TransitTimes holds the standard (sea-level) sun event times for one day.
type TransitTimes struct {
	Sunrise       time.Time
	Sunset        time.Time
	SolarNoon     time.Time
	Nadir         time.Time
	DawnCivil     time.Time // civil dawn (sun 6° below horizon)
	DuskCivil     time.Time // civil dusk (sun 6° below horizon)
	GoldenHourEnd time.Time // morning golden hour ends
	GoldenHour    time.Time // evening golden hour starts
}

// Day returns the standard (no-terrain) sun transit times for a given date and location.
// Uses the suncalc library which accounts for atmospheric refraction but not terrain.
func Day(t time.Time, lat, lon float64) TransitTimes {
	times := suncalc.GetTimes(t, lat, lon)
	return TransitTimes{
		Sunrise:       times[suncalc.Sunrise].Value,
		Sunset:        times[suncalc.Sunset].Value,
		SolarNoon:     times[suncalc.SolarNoon].Value,
		Nadir:         times[suncalc.Nadir].Value,
		DawnCivil:     times[suncalc.Dawn].Value,
		DuskCivil:     times[suncalc.Dusk].Value,
		GoldenHourEnd: times[suncalc.GoldenHourEnd].Value,
		GoldenHour:    times[suncalc.GoldenHour].Value,
	}
}

func radToDeg(r float64) float64 {
	return r * 180.0 / math.Pi
}
