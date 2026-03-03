# Multi-stage build for vcluster-orchestrator-v2 configure pipeline
# Build context: promises/ (to access _shared/kratixutil)
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy shared module first (for caching)
COPY _shared/kratixutil/ ./_shared/kratixutil/

# Copy promise-specific go mod files
COPY vcluster-orchestrator-v2/workflows/resource/configure/go.mod ./vcluster-orchestrator-v2/workflows/resource/configure/
WORKDIR /workspace/vcluster-orchestrator-v2/workflows/resource/configure
RUN go mod tidy && go mod download

# Copy source code
COPY vcluster-orchestrator-v2/workflows/resource/configure/*.go ./

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
COPY --from=builder /workspace/vcluster-orchestrator-v2/workflows/resource/configure/configure /usr/local/bin/configure

# Run as non-root
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/configure"]
