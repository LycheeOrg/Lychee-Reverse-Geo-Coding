package main

import (
	"testing"

	"github.com/authenticvision/rgeo"
)

func TestBuildDisplayName(t *testing.T) {
	tests := []struct {
		name                    string
		city, province, country string
		want                    string
	}{
		{"all present", "Paris", "Île-de-France", "France", "Paris, Île-de-France, France"},
		{"no city", "", "Île-de-France", "France", "Île-de-France, France"},
		{"country only", "", "", "France", "France"},
		{"nothing", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDisplayName(tt.city, tt.province, tt.country)
			if got != tt.want {
				t.Errorf("buildDisplayName(%q, %q, %q) = %q, want %q", tt.city, tt.province, tt.country, got, tt.want)
			}
		})
	}
}

func TestPlaceID(t *testing.T) {
	a := placeID("France", "Île-de-France", "Paris")
	b := placeID("France", "Île-de-France", "Paris")
	if a != b {
		t.Errorf("placeID is not deterministic: %d != %d", a, b)
	}
	if a < 0 {
		t.Errorf("placeID returned negative value: %d", a)
	}

	c := placeID("Japan", "Tokyo", "Tokyo")
	if a == c {
		t.Errorf("placeID collided for distinct inputs: %d", a)
	}

	// Empty location should still produce a stable, non-panicking value.
	if placeID("", "", "") != placeID("", "", "") {
		t.Error("placeID is not deterministic for empty inputs")
	}
}

func TestClampLat(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{0, 0},
		{90, 90},
		{-90, -90},
		{95, 90},
		{-95, -90},
	}
	for _, tt := range tests {
		if got := clampLat(tt.in); got != tt.want {
			t.Errorf("clampLat(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestClampLon(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{0, 0},
		{180, 180},
		{-180, -180},
		{185, 180},
		{-185, -180},
	}
	for _, tt := range tests {
		if got := clampLon(tt.in); got != tt.want {
			t.Errorf("clampLon(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatCoord(t *testing.T) {
	if got := formatCoord(2.3522); got != "2.3522000" {
		t.Errorf("formatCoord(2.3522) = %q, want %q", got, "2.3522000")
	}
}

func TestBoundingBox(t *testing.T) {
	bb := boundingBox(0, 0, 1.0)
	want := [4]string{"-1.0000000", "1.0000000", "-1.0000000", "1.0000000"}
	if bb != want {
		t.Errorf("boundingBox(0, 0, 1.0) = %v, want %v", bb, want)
	}

	// Near the poles/antimeridian the box must clamp instead of wrapping
	// or exceeding valid lat/lon ranges.
	bb = boundingBox(89.5, 179.5, 1.0)
	want = [4]string{"88.5000000", "90.0000000", "178.5000000", "180.0000000"}
	if bb != want {
		t.Errorf("boundingBox(89.5, 179.5, 1.0) = %v, want %v", bb, want)
	}
}

func TestToLower(t *testing.T) {
	tests := map[string]string{
		"FR":  "fr",
		"jp":  "jp",
		"Us":  "us",
		"":    "",
		"A1b": "a1b",
	}
	for in, want := range tests {
		if got := toLower(in); got != want {
			t.Errorf("toLower(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildReverseResponse(t *testing.T) {
	loc := rgeo.Location{
		Country:      "France",
		CountryCode2: "FR",
		Province:     "Île-de-France",
		City:         "Paris",
	}

	t.Run("full detail at zoom 18", func(t *testing.T) {
		resp := buildReverseResponse(loc, 48.8566, 2.3522, 18)
		if resp.Address.City != "Paris" || resp.Address.State != "Île-de-France" || resp.Address.Country != "France" {
			t.Errorf("unexpected address at zoom 18: %+v", resp.Address)
		}
		if resp.Address.CountryCode != "fr" {
			t.Errorf("country_code not lowercased: %q", resp.Address.CountryCode)
		}
		if resp.DisplayName != "Paris, Île-de-France, France" {
			t.Errorf("unexpected display_name: %q", resp.DisplayName)
		}
		if resp.Licence == "" || resp.Lat == "" || resp.Lon == "" {
			t.Error("licence/lat/lon must always be populated")
		}
	})

	t.Run("city dropped below zoom 10", func(t *testing.T) {
		resp := buildReverseResponse(loc, 48.8566, 2.3522, 9)
		if resp.Address.City != "" {
			t.Errorf("expected city to be dropped at zoom 9, got %q", resp.Address.City)
		}
		if resp.Address.State != "Île-de-France" {
			t.Errorf("expected state to survive at zoom 9, got %q", resp.Address.State)
		}
	})

	t.Run("only country survives below zoom 6", func(t *testing.T) {
		resp := buildReverseResponse(loc, 48.8566, 2.3522, 3)
		if resp.Address.City != "" || resp.Address.State != "" {
			t.Errorf("expected city and state dropped at zoom 3, got %+v", resp.Address)
		}
		if resp.Address.Country != "France" {
			t.Errorf("expected country to survive at zoom 3, got %q", resp.Address.Country)
		}
		if resp.DisplayName != "France" {
			t.Errorf("unexpected display_name at zoom 3: %q", resp.DisplayName)
		}
	})

	t.Run("place_id stable for identical location", func(t *testing.T) {
		r1 := buildReverseResponse(loc, 48.8566, 2.3522, 18)
		r2 := buildReverseResponse(loc, 48.8566, 2.3522, 18)
		if r1.PlaceID != r2.PlaceID {
			t.Errorf("place_id not stable: %d != %d", r1.PlaceID, r2.PlaceID)
		}
	})
}
