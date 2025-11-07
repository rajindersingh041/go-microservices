# ---- Build Stage ----
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# --- BUILD ALL BINARIES ---
RUN CGO_ENABLED=0 go build -o /bin/ingest-events        ./cmd/ingest-events/main.go
RUN CGO_ENABLED=0 go build -o /bin/ingest-marketdata    ./cmd/ingest-marketdata/main.go
RUN CGO_ENABLED=0 go build -o /bin/query-events         ./cmd/query-events/main.go
RUN CGO_ENABLED=0 go build -o /bin/query-marketdata     ./cmd/query-marketdata/main.go
RUN CGO_ENABLED=0 go build -o /bin/market-poller        ./cmd/market-poller/main.go
RUN CGO_ENABLED=0 go build -o /bin/internal-transformer ./cmd/internal-transformer/main.go
RUN CGO_ENABLED=0 go build -o /bin/on-demand-fetcher    ./cmd/on-demand-fetcher/main.go

# ---- Final Stage ----
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata curl

# --- COPY ALL BINARIES ---
COPY --from=builder /bin/ingest-events        /bin/ingest-events
COPY --from=builder /bin/ingest-marketdata    /bin/ingest-marketdata
COPY --from=builder /bin/query-events         /bin/query-events
COPY --from=builder /bin/query-marketdata     /bin/query-marketdata
COPY --from=builder /bin/market-poller        /bin/market-poller
COPY --from=builder /bin/internal-transformer /bin/internal-transformer
COPY --from=builder /bin/on-demand-fetcher    /bin/on-demand-fetcher