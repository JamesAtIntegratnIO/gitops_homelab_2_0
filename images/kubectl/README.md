# Custom kubectl Image

A minimal kubectl image based on Alpine Linux with bash support for use in Kubernetes Jobs and CronJobs that require shell scripting.

## Features

- Based on Alpine Linux (minimal size)
- Includes kubectl binary
- Includes bash for script execution
- Runs as non-root user (UID 65534)
- Multi-arch support (amd64, arm64)

## Building

The image is automatically built and published to GitHub Container Registry via GitHub Actions when changes are pushed to the `images/kubectl/` directory.

### Manual Build

```bash
docker build -t ghcr.io/jamesatintegratnio/gitops_homelab_2_0/kubectl:v1.34.1 \
  --build-arg KUBECTL_VERSION=v1.34.1 \
  ./images/kubectl
```

### Trigger Build via GitHub Actions

Go to Actions → "Build and Publish kubectl Image" → Run workflow and optionally specify a kubectl version.

## Usage

```yaml
image:
  registry: ghcr.io
  repository: jamesatintegratnio/gitops_homelab_2_0/kubectl
  tag: v1.34.1
```

## Why Custom Image?

- **registry.k8s.io/kubectl**: Distroless image without shell (no bash/sh)
- **bitnami/kubectl**: No longer publishing new versions to Docker Hub
- **Our image**: Minimal Alpine base with kubectl + bash, actively maintained

## Security

- Runs as non-root user (nobody:nobody, UID 65534)
- Minimal attack surface (Alpine base + kubectl + bash only)
- Regularly updated via automated builds
- Published to private GitHub Container Registry
