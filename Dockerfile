# Use official Go image for building
FROM golang:1.24 AS builder

WORKDIR /app

# Install Node.js for frontend build
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash - \
    && apt-get install -y nodejs make

# Copy source
COPY . .

# Build Maxclaw binary
RUN make build

# Final runtime image
FROM debian:stable-slim

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/build/maxclaw /usr/local/bin/maxclaw
COPY --from=builder /app/build/maxclaw-gateway /usr/local/bin/maxclaw-gateway

# Expose default port
EXPOSE 18890

# Run gateway by default
CMD ["maxclaw-gateway", "-p", "18890"]
