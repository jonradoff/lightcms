# Build stage
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o lightcms ./cmd/server

# Runtime stage (Debian for Cloudflare WARP support)
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl gnupg dbus bash \
    && curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg \
       | gpg --dearmor -o /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ bookworm main" \
       > /etc/apt/sources.list.d/cloudflare-client.list \
    && apt-get update && apt-get install -y --no-install-recommends cloudflare-warp \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/lightcms .

# Copy static files and templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/build.json ./build.json
COPY --from=builder /app/.well-known ./.well-known

# Copy startup script
COPY start.sh ./start.sh
RUN chmod +x start.sh

# Create directories for runtime data
RUN mkdir -p content/generated static/uploads

# Expose port
EXPOSE 8082

# Start WARP daemon + LightCMS
CMD ["./start.sh"]
