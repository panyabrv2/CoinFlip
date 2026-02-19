# --- build stage ---
FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY ../../../Desktop .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/coinflip-server ./cmd/coinflip-server

# --- run stage ---
FROM alpine:3.20
WORKDIR /app

RUN adduser -D -H -s /sbin/nologin appuser
COPY --from=builder /out/coinflip-server /app/coinflip-server

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/coinflip-server"]
