# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o lightcms ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS connections (MongoDB Atlas)
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/lightcms .

# Copy static files and templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/build.json ./build.json

# Create directories for runtime data
RUN mkdir -p content/generated static/uploads

# Expose port
EXPOSE 8082

# Run the binary
CMD ["./lightcms"]
