# syntax=docker/dockerfile:1

# --- Stage 1: build the web UI ---
FROM node:24-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json* ./web/
RUN cd web && npm ci
# The committed placeholder so Vite's emptyOutDir target exists.
COPY internal/api/dist/index.html ./internal/api/dist/index.html
COPY web/ ./web/
RUN cd web && npm run build

# --- Stage 2: build the Go binary (embeds the UI from stage 1) ---
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/api/dist ./internal/api/dist
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /redtrace ./cmd/redtrace

# --- Stage 3: minimal runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /redtrace /usr/local/bin/redtrace
# Inside a container we must bind to all interfaces; the user maps the ports.
EXPOSE 8080 9090
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/redtrace"]
CMD ["serve", "--data-dir", "/data", "--proxy-listen", "0.0.0.0:8080", "--api-listen", "0.0.0.0:9090"]
