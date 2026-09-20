# Sensor Metadata API

A JSON REST API, written in Go on the standard-library `net/http` router with
no web framework; the only third-party runtime dependency is the Swagger UI
handler. Stores and queries sensor metadata: a unique **name**, a GPS
**location** and a list of **tags**. Includes nearest-sensor lookup by
great-circle distance.

## Quick start

```bash
make run                       # serves on :8080
open http://localhost:8080/swagger/   # interactive API docs
```

Or with Docker:

```bash
make docker && docker run --rm -p 8080:8080 sensor-api
```

### Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | TCP port to listen on |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `10s` | Grace period for in-flight requests on SIGINT/SIGTERM |
| `READ_HEADER_TIMEOUT` | `5s` | Slowloris protection |

## API

Base path `/v1`. Bodies are JSON; `Content-Type: application/json` is required on
requests with a body. Unknown fields are rejected. Bodies are capped at 1 MiB.

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/sensors` | Create a sensor (`201`, `Location` header) |
| `GET` | `/v1/sensors` | List sensors, optional `?tag=` filter |
| `GET` | `/v1/sensors/nearest?lat=&lon=` | Sensor closest to a coordinate |
| `GET` | `/v1/sensors/{name}` | Fetch one sensor |
| `PUT` | `/v1/sensors/{name}` | Replace location + tags |
| `PATCH` | `/v1/sensors/{name}` | Update any subset of location / tags |
| `DELETE` | `/v1/sensors/{name}` | Remove a sensor (`204`) |
| `GET` | `/healthz`, `/readyz` | Probes |
| `GET` | `/swagger/` | Swagger UI (`/swagger/doc.json` for the raw spec) |

### Examples

```bash
# create
curl -s -X POST localhost:8080/v1/sensors -H 'Content-Type: application/json' -d '{
  "name": "roof-temp-01",
  "location": {"latitude": 51.5074, "longitude": -0.1278},
  "tags": ["temperature", "roof"]
}'

# read
curl -s localhost:8080/v1/sensors/roof-temp-01

# partial update
curl -s -X PATCH localhost:8080/v1/sensors/roof-temp-01 -H 'Content-Type: application/json' \
  -d '{"tags": ["temperature", "roof", "calibrated"]}'

# nearest
curl -s 'localhost:8080/v1/sensors/nearest?lat=51.5&lon=-0.1'
# {"sensor":{"name":"roof-temp-01",...},"distance_meters":2092.7}
```

### Validation rules

| Field | Rule |
|---|---|
| `name` | 1–64 chars, `^[a-zA-Z0-9][a-zA-Z0-9._-]*$`, unique, immutable, `nearest` is reserved |
| `location.latitude` | finite, −90 … 90 |
| `location.longitude` | finite, −180 … 180 |
| `tags` | ≤ 32 entries, each 1–64 chars after trimming, duplicates dropped |

### Errors

Every error has the same envelope; `fields` is present for validation failures:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "request is invalid",
    "fields": [{"field": "location.latitude", "message": "must be between -90 and 90"}]
  }
}
```

| Status | Code |
|---|---|
| 400 | `validation_failed`, `invalid_json` |
| 404 | `not_found` |
| 405 | `method_not_allowed` (with `Allow` header) |
| 409 | `already_exists`, `conflict` |
| 413 | `payload_too_large` |
| 415 | `unsupported_media_type` |
| 500 | `internal` (details only in logs, correlate via `X-Request-ID`) |

## Development

```bash
make test          # go test -race ./...
make test-cover    # coverage summary
make lint          # golangci-lint
make swagger       # regenerate docs/ from handler annotations
make check         # everything CI runs
```

Requires Go 1.26 and [golangci-lint](https://golangci-lint.run) v2. `swag` is
pinned as a `go tool` in `go.mod`, so no separate install is needed.

## Design

```
cmd/server            wiring, config, graceful shutdown
internal/httpapi      handlers, DTOs, JSON error mapping, middleware
internal/sensor       domain model, validation, Service, Repository interface
internal/storage      memory implementation + reusable contract test suite
internal/geo          haversine distance
```

- **Storage is in-memory** behind `sensor.Repository`. The behavioural contract
  lives in `internal/storage/storagetest`, so a Postgres/PostGIS backend can be
  added and verified with the same tests. Data does not survive restarts.
- **Nearest** is an O(n) haversine scan with deterministic tie-breaking by name.
  Adequate for thousands of sensors; at larger scale swap in a spatial index or
  PostGIS `<->` KNN.
- **Updates are optimistic**: PUT/PATCH read the sensor, build the new version,
  and write it only if `updated_at` is unchanged, returning `409 conflict`
  otherwise. This prevents lost updates between concurrent PATCHes. The
  comparison uses the in-process nanosecond clock, which is fine for a single
  process; a persistent backend would use an integer version column instead —
  the `Repository` interface isolates that change.
- **Validation** is centralised in the domain package, which aggregates every
  failing field at once. The HTTP layer reports a missing `location`,
  `latitude` or `longitude` first, before domain rules run, since those are
  "required" checks that JSON decoding cannot express (pointer fields).
- **No framework**: Go 1.22+ `http.ServeMux` handles method + path patterns;
  the three middleware (recover, request-id, logging) are ~100 lines. JSON
  `405`s are produced only for `/v1/sensors` and `/v1/sensors/{name}`; other
  paths with a wrong method fall through to the JSON `404` catch-all.

## Future work

- Persistent backend (PostGIS) using the existing contract tests.
- Pagination on `GET /v1/sensors`; k-nearest and `max_distance` on `nearest`.
- Authentication, rate limiting, OpenTelemetry traces.
