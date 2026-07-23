package main

import (
	"strconv"
	"strings"

	"github.com/authenticvision/rgeo"
)

// nominatimAddress mirrors the subset of Nominatim's "address" object that
// geocoder-php/nominatim-provider (used by Lychee) reads. Fields we can't
// populate from rgeo's country/province/city data (road, house_number,
// postcode, suburb, ...) are simply omitted.
type nominatimAddress struct {
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

// nominatimReverseResponse mirrors the fields of a Nominatim /reverse
// jsonv2 response that geocoder-php/nominatim-provider reads.
type nominatimReverseResponse struct {
	PlaceID     int64            `json:"place_id"`
	Licence     string           `json:"licence"`
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	DisplayName string           `json:"display_name"`
	Address     nominatimAddress `json:"address"`
	BoundingBox [4]string        `json:"boundingbox"`
}

type nominatimErrorResponse struct {
	Error string `json:"error"`
}

const licenceNotice = "Data from naturalearthdata.com (public domain), reverse geocoded locally by rgeo. Not affiliated with or sourced from OpenStreetMap."

// buildDisplayName joins the resolved parts from most to least specific,
// matching Nominatim's display_name convention.
func buildDisplayName(city, province, country string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{city, province, country} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// placeID synthesizes a stable, deterministic id from the resolved location
// so repeated lookups of the same place return the same value. Nothing in
// the Lychee/geocoder-php call path relies on this beyond it being present.
func placeID(country, province, city string) int64 {
	h := int64(2166136261)
	for _, r := range country + "|" + province + "|" + city {
		h = (h ^ int64(r)) * 16777619
	}
	if h < 0 {
		h = -h
	}
	return h
}

// boundingBox synthesizes an approximate box around the query point since
// rgeo doesn't expose the matched polygon's extent through its public API.
// marginDeg should roughly track the snapping distance used for the lookup.
func boundingBox(lat, lon, marginDeg float64) [4]string {
	south := clampLat(lat - marginDeg)
	north := clampLat(lat + marginDeg)
	west := clampLon(lon - marginDeg)
	east := clampLon(lon + marginDeg)
	return [4]string{
		formatCoord(south),
		formatCoord(north),
		formatCoord(west),
		formatCoord(east),
	}
}

func clampLat(v float64) float64 {
	if v < -90 {
		return -90
	}
	if v > 90 {
		return 90
	}
	return v
}

func clampLon(v float64) float64 {
	if v < -180 {
		return -180
	}
	if v > 180 {
		return 180
	}
	return v
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', 7, 64)
}

// zoomTrimCity/zoomTrimProvince are the minimum "zoom" (per Nominatim's
// /reverse semantics, see nominatim.org/release-docs/latest/api/Reverse)
// below which the corresponding detail level is dropped from the response.
// rgeo never resolves anything finer than city, so zoom can only ever trim
// detail here, never add it.
const (
	zoomTrimProvince = 6
	zoomTrimCity     = 10
)

// buildReverseResponse assembles a Nominatim-jsonv2-compatible /reverse
// response from a resolved rgeo Location and the query's lat/lon/zoom.
func buildReverseResponse(loc rgeo.Location, lat, lon float64, zoom int) nominatimReverseResponse {
	city := loc.City
	province := loc.Province
	if zoom < zoomTrimCity {
		city = ""
	}
	if zoom < zoomTrimProvince {
		province = ""
	}

	addr := nominatimAddress{
		City:        city,
		State:       province,
		Country:     loc.Country,
		CountryCode: toLower(loc.CountryCode2),
	}

	return nominatimReverseResponse{
		PlaceID:     placeID(loc.Country, province, city),
		Licence:     licenceNotice,
		Lat:         formatCoord(lat),
		Lon:         formatCoord(lon),
		DisplayName: buildDisplayName(city, province, loc.Country),
		Address:     addr,
		BoundingBox: boundingBox(lat, lon, snappingDistanceKM/111.0),
	}
}
