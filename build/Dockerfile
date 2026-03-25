FROM golang:1.24-alpine3.21 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /bin/kintsugi-storage ./cmd/server

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/kintsugi-storage /app/kintsugi-storage

EXPOSE 8080

ENTRYPOINT ["/app/kintsugi-storage"]
