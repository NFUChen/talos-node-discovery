# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o talos-probe .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS and curl for downloading talosctl
RUN apk --no-cache add ca-certificates curl

# Install talosctl
ARG TALOS_VERSION=v1.11.5
RUN curl -sL https://github.com/siderolabs/talos/releases/download/${TALOS_VERSION}/talosctl-linux-amd64 -o /usr/local/bin/talosctl && \
    chmod +x /usr/local/bin/talosctl

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/talos-probe .

# Create directory for config files
RUN mkdir -p /app/config

# The application expects: <cidr> <talosconfig path> <worker config path> <batch size>
# Example: talos-probe 192.168.1.0/24 /app/config/talosconfig /app/config/worker.yaml 10
ENTRYPOINT ["/app/talos-probe"]

