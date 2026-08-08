# Trawl on Cloud Run — server and dashboard in one image.
#
# Cloud Run serves a single container per service, so the Angular bundle is
# served by the Go binary rather than by a separate nginx container. That also
# removes the need to proxy /api and the event stream: the dashboard and the
# API are the same origin because they are the same process.

FROM node:24-alpine AS frontend

WORKDIR /app
COPY app/package*.json ./
RUN npm ci
COPY app/ ./
RUN npm run build -- --configuration=production

FROM golang:1.26-alpine AS build

WORKDIR /src

# Pure-Go SQLite (modernc.org/sqlite) means no cgo and no build toolchain.
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
# config/ carries the signal registry, embedded at compile time.
COPY config/ ./config/

# The dashboard is embedded into the binary, so there is no second image to
# keep in step and no version skew between the API and the UI that calls it.
COPY --from=frontend /app/dist/trawl-app/browser ./cmd/trawl/webroot/

ARG VERSION=dev
ARG COMMIT=""
ARG BUILD_DATE=""
RUN go build -trimpath \
    -ldflags="-s -w \
      -X github.com/adedayo/trawl/pkg/version.Version=${VERSION} \
      -X github.com/adedayo/trawl/pkg/version.Commit=${COMMIT} \
      -X github.com/adedayo/trawl/pkg/version.BuildDate=${BUILD_DATE}" \
    -tags embed_dashboard -o /out/trawl-server ./cmd/trawl

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 trawl \
    && mkdir -p /data \
    && chown trawl:trawl /data

COPY --from=build /out/trawl-server /usr/local/bin/trawl-server

USER trawl

# TRAWL_DB_DSN selects the storage backend. A bare path is taken as SQLite and
# points at the mount path of a Cloud Run volume. Without a mounted volume this
# path is instance-local in-memory storage: it counts against the instance's
# memory limit and is discarded when the instance is reclaimed. Setting a
# scheme-qualified DSN (for example libsql://…) selects a networked backend
# instead, which is what lifts the max-instances=1 constraint.
# See deploy/cloudrun/README.md.
ENV TRAWL_DB_DSN=/data/trawl.db

# PORT is injected by Cloud Run and takes precedence over TRAWL_LISTEN_ADDR.
EXPOSE 8080

ENTRYPOINT ["trawl-server", "server"]
