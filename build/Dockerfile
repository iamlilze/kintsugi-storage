FROM golang:1.24-alpine3.21 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /bin/kintsugi-storage ./cmd/server

FROM scratch

WORKDIR /app

COPY --from=builder /bin/kintsugi-storage /app/kintsugi-storage

EXPOSE 8080

ENV HTTP_ADDR=:8080
ENV SNAPSHOT_PATH=/app/data/snapshot.json
ENV CLEANUP_INTERVAL=5s
ENV SNAPSHOT_INTERVAL=30s
ENV SHUTDOWN_TIMEOUT=10s
ENV MAX_BODY_BYTES=1048576
ENV LOG_LEVEL=info
ENV LOG_FORMAT=json

ENTRYPOINT ["/app/kintsugi-storage"]