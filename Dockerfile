# ---- Build Stage ----
# Use the official Go image as the builder
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy module files and download dependencies
# This is done first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build both service binaries
RUN CGO_ENABLED=0 go build -o /bin/ingestion-service ./cmd/ingestion-service/main.go
RUN CGO_ENABLED=0 go build -o /bin/query-service ./cmd/query-service/main.go

# ---- Final Stage ----
# Use a minimal Alpine image for the final container
FROM alpine:latest

# Add certificates for any potential outbound HTTPS calls
RUN apk --no-cache add ca-certificates

# Copy *only* the compiled binaries from the builder stage
COPY --from=builder /bin/ingestion-service /bin/ingestion-service
COPY --from=builder /bin/query-service /bin/query-service

# We will specify the command to run in docker-compose.yml