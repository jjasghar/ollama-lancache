# Multi-stage build for ollama-lancache
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ollama-lancache .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create a non-root user
RUN addgroup -g 1001 -S ollama && \
    adduser -u 1001 -S ollama -G ollama

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/ollama-lancache .

# Copy client scripts
COPY --from=builder /app/scripts ./scripts

# Create directories for models and cache
RUN mkdir -p /models /cache && \
    chown -R ollama:ollama /app /models /cache

# Switch to non-root user
USER ollama

# Expose ports
# 8080 for model distribution server
# 80 for registry proxy HTTP
# 53 for DNS (if running as root)
EXPOSE 8080 80 53/udp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/info || curl -f http://localhost:80/health || exit 1

# Default command - run model distribution server
CMD ["./ollama-lancache", "serve", "--port", "8080", "--models-dir", "/models"]

# Labels for metadata
LABEL maintainer="JJ Asghar <jjasghar@gmail.com>"
LABEL description="Ollama LanCache - Model distribution system for Ollama"
LABEL version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/jjasghar/ollama-lancache"
LABEL org.opencontainers.image.documentation="https://github.com/jjasghar/ollama-lancache/blob/main/README.md"
LABEL org.opencontainers.image.licenses="MIT"
