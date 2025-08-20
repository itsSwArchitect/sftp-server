FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for some Go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/sftpd ./cmd/sftpd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/sftpd .

# Copy web assets
COPY --from=builder /app/web ./web

# Copy config files (if any)
COPY --from=builder /app/configs ./configs

# Create non-root user
RUN addgroup -g 1001 -S sftpuser && \
    adduser -S sftpuser -u 1001 -G sftpuser

# Change ownership of app directory
RUN chown -R sftpuser:sftpuser /app

# Switch to non-root user
USER sftpuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./sftpd", "-h", "0.0.0.0", "-p", "8080"]
