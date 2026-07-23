# Lychee Reverse Geo Coding

[![CI](https://github.com/LycheeOrg/Lychee-Reverse-Geo-Coding/actions/workflows/ci.yml/badge.svg)](https://github.com/LycheeOrg/Lychee-Reverse-Geo-Coding/actions/workflows/ci.yml)
[![OpenSSF Scorecard][ossf-shield]](https://securityscorecards.dev/viewer/?uri=github.com/LycheeOrg/Lychee-Reverse-Geo-Coding)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/2855/badge)](https://www.bestpractices.dev/projects/2855)

Self-hosted reverse geocoding service exposing a Nominatim-`/reverse`-compatible
endpoint, backed entirely by data embedded in the binary via
[authenticvision/rgeo](https://github.com/authenticvision/rgeo) (Natural Earth
country/province/city polygons). No outbound network calls, no rate limits, no
API key.

Built as a drop-in replacement for `https://nominatim.openstreetmap.org` for
callers that only need `/reverse`, such as [Lychee](https://github.com/LycheeOrg/Lychee).

## Data granularity

rgeo resolves to **country / province / city** level only (point-in-polygon
against Natural Earth data), not street/house-number level. `road`,
`house_number`, `postcode`, `suburb`, etc. are never present in the response —
they're simply omitted, which the Nominatim client libraries treat as normal
optional fields. `display_name` and `address.{city,state,country,country_code}`
are always populated when a match is found.

## API

```
GET /reverse?lat={float}&lon={float}&format=jsonv2&addressdetails=1&zoom={0-18}
```

- `lat`, `lon` — required, decimal degrees.
- `zoom` — optional (default 18), trims the response like real Nominatim:
  `zoom < 6` → country only, `zoom < 10` → country + state, `zoom >= 10` →
  country + state + city.
- `format`, `addressdetails` — accepted for compatibility, not otherwise used.

Coordinates that don't land in or within ~5km of a landmass (open ocean)
return `200 {"error": "Unable to geocode"}`, matching real Nominatim's
behavior. Missing/invalid `lat`/`lon` return `400`.

`GET /health` returns `200` for container healthchecks.

`GET /openapi.json` serves the [OpenAPI 3.0 spec](docs/openapi.json) for this
API — paste it into [editor.swagger.io](https://editor.swagger.io) or any
local Swagger/Redoc viewer to browse it interactively.

## Running

```bash
docker compose up -d --build
```

The service listens on `:8080` (override with `LISTEN_ADDR`). No volumes are
needed — the geocoding datasets are compiled into the binary.

## Pointing Lychee at it

Configure Lychee's Nominatim root URL to this service's address (e.g.
`http://geo-decoding:8080` on the shared compose network) instead of
`https://nominatim.openstreetmap.org`.

## Development

Requires Go 1.26+ (or `GOTOOLCHAIN=auto`, the default, to fetch it
automatically — see [go.mod](go.mod) for the pinned version).

```bash
make build       # go build -o geo-decoding-server .
make run         # build + run locally on :8080
make vet         # go vet ./...
make fmt-check   # gofmt -l . (fails if anything is unformatted)
make test        # go test ./...
make test-race   # go test -race ./...
make all         # build + vet + fmt-check + test
make docker-build
make docker-run  # docker run -p 8080:8080
```

Tests build the real rgeo dataset once in `TestMain` (a few seconds) and then
exercise the HTTP handlers via `httptest` — no network access or Docker
required.

[ossf-shield]: https://api.securityscorecards.dev/projects/github.com/LycheeOrg/Lychee-Reverse-Geo-Coding/badge
