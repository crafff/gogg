# syntax=docker/dockerfile:1.7

# ── Build stage ─────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache git ca-certificates tzdata && \
    update-ca-certificates

WORKDIR /workspace

# Copy module manifests first to maximise layer cache reuse.
COPY go.work go.work.sum* ./
COPY go.mod go.sum ./
# When apps/api becomes its own module, switch to:
#   COPY apps/api/go.mod apps/api/go.sum apps/api/
#   COPY packages/*/go.mod packages/*/go.sum packages/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

# CGO off + trimpath + ldflags for a fully static, reproducible
# binary. The placeholder cmd path is updated once apps/api lands.
ARG CMD_PATH=./apps/api/cmd/api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/gogg-api \
      ${CMD_PATH}

# ── Runtime stage ───────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

LABEL org.opencontainers.image.title="gogg-api" \
      org.opencontainers.image.description="GOGG GraphQL BFF + REST compat" \
      org.opencontainers.image.source="https://github.com/crafff/gogg" \
      org.opencontainers.image.licenses="UNLICENSED"

WORKDIR /app
COPY --from=builder /out/gogg-api /app/gogg-api
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/gogg-api"]
