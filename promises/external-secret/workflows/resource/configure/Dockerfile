# Multi-stage build for external-secret configure pipeline
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy shared module first (for caching)
COPY _shared/kratixutil/ ./_shared/kratixutil/

# Copy promise-specific go mod files
COPY external-secret/workflows/resource/configure/go.mod ./external-secret/workflows/resource/configure/
WORKDIR /workspace/external-secret/workflows/resource/configure
RUN go mod tidy && go mod download

# Copy source code
COPY external-secret/workflows/resource/configure/*.go ./

# Build static binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o configure \
    .

# Runtime stage
FROM --platform=$TARGETPLATFORM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy binary from builder
COPY --from=builder /workspace/external-secret/workflows/resource/configure/configure /usr/local/bin/configure

# Run as non-root
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/configure"]
