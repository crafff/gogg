# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache git ca-certificates tzdata && \
    update-ca-certificates

WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ARG CMD_PATH=./apps/worker/cmd/worker
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/gogg-worker \
      ${CMD_PATH}

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

LABEL org.opencontainers.image.title="gogg-worker" \
      org.opencontainers.image.description="GOGG Temporal worker (crawler + async jobs)" \
      org.opencontainers.image.source="https://github.com/crafff/gogg" \
      org.opencontainers.image.licenses="UNLICENSED"

WORKDIR /app
COPY --from=builder /out/gogg-worker /app/gogg-worker
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nonroot:nonroot
ENTRYPOINT ["/app/gogg-worker"]
