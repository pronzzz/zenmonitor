# Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is needed for go-sqlite3 if we use it, 
# but modernc.org/sqlite is CGO-free. PRD mentioned CGO-free preferred.
# We will assume modernc.org/sqlite for now to keep it simple and small.
RUN CGO_ENABLED=0 GOOS=linux go build -o zenmonitor ./cmd/server

# Run Stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (check if CA certs needed)
RUN apk add --no-cache ca-certificates tzdata

# Create directory for data and config
RUN mkdir -p /app/data

# Copy binary from builder
COPY --from=builder /app/zenmonitor .
COPY --from=builder /app/web ./web

# Copy default config if not mounted (optional)
# COPY monitors.yaml /app/data/

# Expose port
EXPOSE 8080

# Volume for persistence and config
VOLUME ["/app/data", "/app/config"]

# Command to run
CMD ["./zenmonitor"]
