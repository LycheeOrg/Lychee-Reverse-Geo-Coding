package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/authenticvision/rgeo"
	"github.com/gin-gonic/gin"
	"github.com/twpayne/go-geom"
)

// snappingDistanceKM controls how far ReverseGeocodeSnapping will look for
// the nearest landmass when a coordinate doesn't fall directly inside a
// polygon (e.g. GPS noise placing a photo just offshore).
const snappingDistanceKM = 5.0

//go:embed docs/openapi.json
var openAPISpec []byte

func main() {
	addr := flag.String("addr", envOr("LISTEN_ADDR", ":8080"), "address to listen on")
	healthcheck := flag.Bool("healthcheck", false, "perform an HTTP healthcheck against -addr and exit (used by Docker HEALTHCHECK)")
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck(*addr))
	}

	log.Println("loading reverse geocoding datasets...")
	geo, err := rgeo.New(rgeo.Cities10, rgeo.Provinces10)
	if err != nil {
		log.Fatalf("failed to initialize rgeo: %v", err)
	}
	geo.Build()
	geo.SetSnappingDistanceEarth(snappingDistanceKM)
	log.Println("datasets loaded")

	r := newRouter(geo)

	log.Printf("listening on %s", *addr)
	if err := r.Run(*addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// newRouter builds the Gin engine and registers all routes. Split out from
// main so tests can exercise the HTTP layer with httptest without going
// through flag parsing or r.Run.
func newRouter(geo *rgeo.Rgeo) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", openAPISpec)
	})
	r.GET("/reverse", reverseHandler(geo))

	return r
}

func reverseHandler(geo *rgeo.Rgeo) gin.HandlerFunc {
	return func(c *gin.Context) {
		latStr := c.Query("lat")
		lonStr := c.Query("lon")
		if latStr == "" || lonStr == "" {
			c.JSON(http.StatusBadRequest, nominatimErrorResponse{Error: "missing required parameters: lat, lon"})
			return
		}

		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil || lat < -90 || lat > 90 {
			c.JSON(http.StatusBadRequest, nominatimErrorResponse{Error: "invalid lat parameter"})
			return
		}

		lon, err := strconv.ParseFloat(lonStr, 64)
		if err != nil || lon < -180 || lon > 180 {
			c.JSON(http.StatusBadRequest, nominatimErrorResponse{Error: "invalid lon parameter"})
			return
		}

		zoom := 18
		if zoomStr := c.Query("zoom"); zoomStr != "" {
			if z, err := strconv.Atoi(zoomStr); err == nil {
				zoom = z
			}
		}

		loc, err := geo.ReverseGeocodeSnapping(geom.Coord{lon, lat})
		if err != nil {
			if errors.Is(err, rgeo.ErrLocationNotFound) {
				c.JSON(http.StatusOK, nominatimErrorResponse{Error: "Unable to geocode"})
				return
			}
			c.JSON(http.StatusInternalServerError, nominatimErrorResponse{Error: "internal geocoding error"})
			return
		}

		c.JSON(http.StatusOK, buildReverseResponse(loc, lat, lon, zoom))
	}
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runHealthcheck performs a plain HTTP GET against /health on addr and
// returns a process exit code. It exists so a scratch-based container
// (no shell, no curl/wget) can still run `HEALTHCHECK CMD ["/server", "-healthcheck"]`
// by invoking the same static binary with a different flag.
func runHealthcheck(addr string) int {
	host := addr
	if len(host) > 0 && host[0] == ':' {
		host = "127.0.0.1" + host
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", host))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck failed: status", resp.StatusCode)
		return 1
	}
	return 0
}
