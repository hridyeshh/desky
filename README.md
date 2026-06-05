# desky

Go backend for a Raspberry Pi desk display. Stores the screen config and serves
widget data. A Pi Zero 2W with 3 SPI displays polls this every 5s to learn which
widget to show on each screen; an iPhone app pushes config updates.

Stack: Go (net/http standard library) + Postgres. Deployed on Railway.

## Data model

One config row, three screen slots. Each slot holds one widget name:

```
clock  spotify  weather  tasks  gif
```

Default seed on first run: `screen1=clock`, `screen2=gif`, `screen3=spotify`.

## Local setup

Requires Go 1.22+ and a Postgres database.

```bash
# 1. copy env template and fill in values
cp .env.example .env

# 2. (optional) run Postgres locally via Docker
docker run -d --name desky-pg \
  -e POSTGRES_USER=desky -e POSTGRES_PASSWORD=desky -e POSTGRES_DB=desky \
  -p 5433:5432 postgres:16-alpine
# then set in .env:
# DATABASE_URL=postgres://desky:desky@localhost:5433/desky?sslmode=disable

# 3. run (migrations + seed run automatically on startup)
make run      # go run ./cmd
make build    # builds ./bin/desky
```

Server listens on `PORT` (default `8080`).

## Environment variables

| Variable              | Required | Description                                                        |
|-----------------------|----------|--------------------------------------------------------------------|
| `DATABASE_URL`        | yes      | Postgres connection string. Railway provides this automatically.   |
| `PORT`                | no       | HTTP listen port. Railway sets this; defaults to `8080` locally.   |
| `OPENWEATHER_API_KEY` | no\*     | OpenWeatherMap free-tier key. \*Required for `GET /widget/weather`. |

## API

All responses are JSON. CORS is open to all origins (`*`).

### `GET /health`
```json
{"status": "ok"}
```

### `GET /config`
Current screen config.
```json
{"screen1": "clock", "screen2": "gif", "screen3": "spotify"}
```

### `PUT /config`
Partial update — send any subset of the three slots. Returns the full updated
config. Rejects unknown widget names with `400`.
```bash
curl -X PUT $HOST/config -d '{"screen1":"weather"}'
```
```json
{"screen1": "weather", "screen2": "gif", "screen3": "spotify"}
```

### `GET /widget/clock`
Current time in IST (Asia/Kolkata).
```json
{"time": "14:32", "date": "SAT 06 JUN", "day": "Saturday"}
```

### `GET /widget/weather`
Current Bangalore weather (lat 12.9716, lon 77.5946) via OpenWeatherMap
current-weather endpoint. Needs `OPENWEATHER_API_KEY`.
```json
{"temp": "28°C", "condition": "Clouds", "humidity": "72%", "icon": "cloud"}
```
`icon` is one of: `sun cloud rain storm snow fog`.

### `GET /widget/spotify`
Placeholder until Phase 4 wiring (Spotify/Last.fm).
```json
{"status": "not_configured", "track": "", "artist": ""}
```

## Deploy on Railway

1. Push this repo to GitHub, create a Railway project from it.
2. Add a Railway **Postgres** plugin — it injects `DATABASE_URL` automatically.
3. Set `OPENWEATHER_API_KEY` in the service variables.
4. Railway sets `PORT` and runs the build/start from `railway.toml`
   (`go build -o bin/desky ./cmd` → `./bin/desky`).

Migrations and the default-config seed run automatically on startup.
