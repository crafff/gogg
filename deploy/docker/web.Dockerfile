# syntax=docker/dockerfile:1.7

# ── Build stage ─────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /workspace/apps/web

# Manifest-only layer for maximal cache reuse.
COPY apps/web/package.json apps/web/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY apps/web/ ./
# VITE_API_BASE_URL is baked into the bundle at build time; pass
# via --build-arg in CI per environment.
ARG VITE_API_BASE_URL=/api
ARG VITE_APP_ENV=production
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL} \
    VITE_APP_ENV=${VITE_APP_ENV}

RUN npm run build

# ── Runtime stage ───────────────────────────────────────────
FROM nginx:1.27-alpine AS runtime

LABEL org.opencontainers.image.title="gogg-web" \
      org.opencontainers.image.description="GOGG React frontend" \
      org.opencontainers.image.source="https://github.com/crafff/gogg" \
      org.opencontainers.image.licenses="UNLICENSED"

# Replace the default config with one that does SPA fallback and
# caches hashed assets aggressively.
COPY deploy/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/apps/web/dist /usr/share/nginx/html

EXPOSE 80

# nginx already runs as root and drops to nginx user for workers,
# matching the upstream image's expectations.
