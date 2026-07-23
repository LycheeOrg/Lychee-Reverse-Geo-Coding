package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/authenticvision/rgeo"
)

var testGeo *rgeo.Rgeo

// TestMain builds the rgeo dataset once for the whole package instead of
// per-test: loading it takes several seconds, and it's read-only after Build.
func TestMain(m *testing.M) {
	geo, err := rgeo.New(rgeo.Cities10, rgeo.Provinces10)
	if err != nil {
		panic(err)
	}
	geo.Build()
	geo.SetSnappingDistanceEarth(snappingDistanceKM)
	testGeo = geo

	os.Exit(m.Run())
}

func TestHealthEndpoint(t *testing.T) {
	r := newRouter(testGeo)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOpenAPIEndpoint(t *testing.T) {
	r := newRouter(testGeo)
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json = %d, want %d", w.Code, http.StatusOK)
	}
	var spec map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if spec["openapi"] == nil {
		t.Error("expected an \"openapi\" version field in the served spec")
	}
}

func TestReverseEndpoint_MissingParams(t *testing.T) {
	r := newRouter(testGeo)
	req := httptest.NewRequest(http.MethodGet, "/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GET /reverse (no params) = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestReverseEndpoint_InvalidCoordinates(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"non-numeric lat", "lat=abc&lon=2.35"},
		{"non-numeric lon", "lat=48.85&lon=xyz"},
		{"lat out of range", "lat=200&lon=2.35"},
		{"lon out of range", "lat=48.85&lon=200"},
	}
	r := newRouter(testGeo)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/reverse?"+tt.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("GET /reverse?%s = %d, want %d", tt.query, w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestReverseEndpoint_Paris(t *testing.T) {
	r := newRouter(testGeo)
	req := httptest.NewRequest(http.MethodGet, "/reverse?format=jsonv2&lat=48.8566&lon=2.3522&addressdetails=1&zoom=18", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /reverse (Paris) = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp nominatimReverseResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Address.Country != "France" {
		t.Errorf("Address.Country = %q, want %q", resp.Address.Country, "France")
	}
	if resp.Address.CountryCode != "fr" {
		t.Errorf("Address.CountryCode = %q, want %q", resp.Address.CountryCode, "fr")
	}
	if resp.DisplayName == "" {
		t.Error("display_name must not be empty for a resolved location")
	}
	if resp.Licence == "" {
		t.Error("licence must always be populated (read unconditionally by geocoder-php)")
	}
	for _, edge := range resp.BoundingBox {
		if edge == "" {
			t.Errorf("boundingbox entries must never be empty, got %v", resp.BoundingBox)
		}
	}
}

func TestReverseEndpoint_ZoomTrimsDetail(t *testing.T) {
	r := newRouter(testGeo)
	req := httptest.NewRequest(http.MethodGet, "/reverse?lat=48.8566&lon=2.3522&zoom=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp nominatimReverseResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Address.City != "" {
		t.Errorf("expected city omitted at zoom=3, got %q", resp.Address.City)
	}
	if resp.Address.Country != "France" {
		t.Errorf("expected country present at zoom=3, got %q", resp.Address.Country)
	}
}

func TestReverseEndpoint_OpenOcean(t *testing.T) {
	r := newRouter(testGeo)
	// Mid-Atlantic, far from any landmass or the 5km snapping radius.
	req := httptest.NewRequest(http.MethodGet, "/reverse?lat=30&lon=-40", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /reverse (ocean) = %d, want %d", w.Code, http.StatusOK)
	}
	var resp nominatimErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected a non-empty error message for an ungeocodable coordinate")
	}
}

func TestRunHealthcheck(t *testing.T) {
	r := newRouter(testGeo)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// srv.Listener.Addr() gives "127.0.0.1:PORT"; runHealthcheck also
	// accepts the ":PORT"-only form used by the -addr flag, exercised here
	// via the full host:port form it falls back to unchanged.
	addr := srv.Listener.Addr().String()
	if code := runHealthcheck(addr); code != 0 {
		t.Errorf("runHealthcheck(%q) = %d, want 0", addr, code)
	}

	if code := runHealthcheck("127.0.0.1:1"); code == 0 {
		t.Error("runHealthcheck against a closed port should fail")
	}
}
