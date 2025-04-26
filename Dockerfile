# Stage 1: Build the application
FROM golang:1.23-alpine AS builder

# Install C compiler and necessary headers for CGo (required by go-sqlite3)
RUN apk add --no-cache build-base

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies (including CGo dependencies)
RUN go mod download

# Copy the source code
COPY . .

# Build the application with CGo enabled
# -ldflags="-w -s" reduces the size of the binary
# The final binary will be statically linked if possible on Alpine
RUN go build -ldflags="-w -s" -o /retro-template ./cmd/app/main.go

# Stage 2: Create the final image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /retro-template /app/retro-template

# Copy configuration and local data directories
# Create directories in case volumes are not mounted initially
RUN mkdir -p /app/config /app/local
COPY config/ /app/config/

# Set the entrypoint
ENTRYPOINT ["/app/retro-template"]

# Requires config and wallets path inside the container
CMD ["--config", "/app/config/config.yml", "--wallets", "/app/local/private_keys.txt"] 