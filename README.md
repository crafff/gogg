# GOGG (Go + React)

This project now includes:

- Go backend server (port `8080` by default)
- React frontend (`web/`) for champion rankings
- PostgreSQL-backed ranking API using:
	`postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable`

## Run Backend

```bash
go mod tidy
go build .
./gogg
```

Environment variables:

- `PORT` (default: `8080`)
- `DATABASE_DSN` (default: `postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable`)
- `WEB_DIST_DIR` (default: `web/dist`)

## Run Frontend (Dev)

```bash
cd web
npm install
npm run dev
```

Dev frontend uses Vite proxy to backend `/api`.

## Build Frontend for Go Static Serving

```bash
cd web
npm run build
```

After build, backend serves frontend at `/` from `web/dist`.

## API

`GET /api/rankings/champions`

Query params:

- `limit` (default `20`, range `1-200`)
- `minGames` (default `20`)
- `position` (`TOP`, `JUNGLE`, `MIDDLE`, `BOTTOM`, `UTILITY`)
- `queueId` (default `420`)
