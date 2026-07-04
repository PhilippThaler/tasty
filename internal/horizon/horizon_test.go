package horizon

import (
	"math"
	"testing"
)

// constantElevProvider returns a fixed elevation everywhere.
type constantElevProvider struct {
	elev float64
}

func (c constantElevProvider) Elevation(lat, lon float64) float64 {
	return c.elev
}

func TestComputeFlatTerrain(t *testing.T) {
	p := Compute(constantElevProvider{elev: 0}, 47.27, 11.40)
	if p.AzSteps != defaultAzimuthSteps {
		t.Fatalf("azimuth steps = %d, want %d", p.AzSteps, defaultAzimuthSteps)
	}
	for i, e := range p.Elevations {
		if math.Abs(e) > 0.1 {
			t.Errorf("azimuth %d: flat horizon = %.2f°, want ~0°", i, e)
		}
	}
}

func TestComputeNoData(t *testing.T) {
	p := Compute(nanProvider{}, 30.0, -39.0)
	if p.AzSteps != defaultAzimuthSteps {
		t.Fatalf("azimuth steps = %d, want %d", p.AzSteps, defaultAzimuthSteps)
	}
	for i, e := range p.Elevations {
		if math.Abs(e) > 0.1 {
			t.Errorf("azimuth %d: no-data horizon = %.2f°, want ~0°", i, e)
		}
	}
}

func TestHorizonElevationLookup(t *testing.T) {
	profile := &Profile{
		Lat:        0,
		Lon:        0,
		Elevation:  0,
		AzSteps:    4,
		Elevations: []float64{0, 10, 20, 30}, // N, E, S, W
	}

	cases := []struct {
		az   float64
		want float64
	}{
		{0, 0},
		{90, 10},
		{180, 20},
		{270, 30},
		{360, 0},  // wrap around
		{-90, 30}, // negative azimuth
	}

	for _, tc := range cases {
		got := profile.HorizonElevation(tc.az)
		if math.Abs(got-tc.want) > 0.01 {
			t.Errorf("HorizonElevation(%.1f°) = %.2f°, want %.2f°", tc.az, got, tc.want)
		}
	}
}

// nanProvider returns NaN everywhere, simulating missing SRTM data.
type nanProvider struct{}

func (nanProvider) Elevation(lat, lon float64) float64 {
	return math.NaN()
}
