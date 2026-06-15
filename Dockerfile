# Build stage
FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Pure-Go build (modernc sqlite + anacrolix torrent need no cgo) -> static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mirrorbot ./cmd/mirrorbot

# Runtime stage (alpine so the compose healthcheck can use wget)
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates
WORKDIR /app
COPY --from=build /out/mirrorbot /usr/local/bin/mirrorbot

# Data, downloads and service-account dirs are expected as mounted volumes.
ENV DOWNLOAD_DIR=/app/downloads \
    SA_DIR=/app/accounts \
    DB_DSN="file:/app/data/mirrorbot.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" \
    HEALTH_ADDR=":7870"

EXPOSE 7870
ENTRYPOINT ["mirrorbot"]
