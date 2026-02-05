# Multi-stage build for vcluster-orchestrator-v2 configure pipeline
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy source code first
COPY main.go ./
COPY templates/ ./templates/

# Copy go mod files
COPY go.mod ./
RUN go mod tidy && go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o configure \
    main.go

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy binary from builder
COPY --from=builder /workspace/configure /usr/local/bin/configure
COPY --from=builder /workspace/templates /templates

# Run as non-root
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/configure"]
