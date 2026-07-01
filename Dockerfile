# Multi-stage Docker build for terrain-sunset
# Stage 1: build Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /terrain-sunset ./cmd/server

# Stage 2: minimal runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 app

WORKDIR /app
COPY --from=builder /terrain-sunset .
COPY web/ ./web/

# SRTM data will be mounted at /data/srtm
RUN mkdir -p /data/srtm && chown app:app /data/srtm

USER app
EXPOSE 8080

ENTRYPOINT ["./terrain-sunset"]
CMD ["-port", "8080", "-data", "/data/srtm", "-web", "./web"]
