# Multi-stage build for http-service configure pipeline
# Build context: promises/ (to access _shared/kratixutil)
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy shared module first (for caching)
COPY _shared/kratixutil/ ./_shared/kratixutil/

# Copy promise-specific go mod and sum files
COPY http-service/workflows/resource/configure/go.mod http-service/workflows/resource/configure/go.sum ./http-service/workflows/resource/configure/
WORKDIR /workspace/http-service/workflows/resource/configure
RUN go mod download

# Copy source code
COPY http-service/workflows/resource/configure/*.go ./

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
COPY --from=builder /workspace/http-service/workflows/resource/configure/configure /usr/local/bin/configure

# Run as non-root
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/configure"]
