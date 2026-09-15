# ==============================================================================
# Multi-Stage Dockerfile for EVA Bharat Media Sequencer (Golang Backend)
# ==============================================================================

# Stage 1: Build binary
FROM golang:1.26.4-alpine AS builder

WORKDIR /app

# Install certificates and git
RUN apk add --no-cache ca-certificates git tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server ./cmd/server/main.go

# Stage 2: Minimal runtime image
FROM alpine:3.21

WORKDIR /app

# Add ca-certificates and tzdata for SSL and UTC timing
RUN apk add --no-cache ca-certificates tzdata

# Create non-privileged user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy only the compiled binary from builder
COPY --from=builder /app/server /app/server

# Expose server port (default 8080)
EXPOSE 8080

# Environment defaults
ENV PORT=8080 \
    ENVIRONMENT=production

# Start binary
ENTRYPOINT ["/app/server"]
