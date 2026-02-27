# Multi-stage build for platform-status-reconciler
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY *.go ./
RUN go mod tidy && go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o reconciler \
    .

# Runtime stage
FROM --platform=$TARGETPLATFORM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy binary from builder
COPY --from=builder /workspace/reconciler /usr/local/bin/reconciler

# Run as non-root
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/reconciler"]
