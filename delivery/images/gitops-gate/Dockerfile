# The gate shells out to helm and kubeconform rather than vendoring their
# libraries: chart rendering has to match what the cluster's own Helm does, and
# pinning a library version is a slower way to drift away from that.
# Deliberately NOT `FROM --platform=$BUILDPLATFORM`, which would cross-compile
# and skip emulating this stage. The legacy builder does not substitute
# $BUILDPLATFORM, so the flag becomes `--platform=` and the build dies with
# "invalid OS component" -- breaking every `docker build` run without buildx,
# which is the common case on a developer machine. A minute of emulated Go
# build in CI is the cheaper trade.
FROM golang:1.26-alpine AS build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH:-$(go env GOARCH)}" \
    go build -trimpath -ldflags='-s -w' -o /out/gitops-gate .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git bash

ARG HELM_VERSION=v3.19.0
ARG KUBECONFORM_VERSION=v0.7.0
# No default value. TARGETARCH is a built-in BuildKit arg, and assigning one
# here SHADOWS the value BuildKit injects -- which made the arm64 image
# download amd64 helm and kubeconform, then fail exec'ing them under emulation.
#
# The fallback below is computed inside RUN instead, so BuildKit stays
# authoritative when it sets the arg, and a plain `docker build` without
# buildx (which injects nothing) still resolves to the native architecture
# rather than fetching `helm-...-linux-.tar.gz` and failing on `tar: invalid
# magic`.
ARG TARGETARCH

RUN set -eux; \
    arch="${TARGETARCH:-$(apk --print-arch | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')}"; \
    wget -qO- "https://get.helm.sh/helm-${HELM_VERSION}-linux-${arch}.tar.gz" \
      | tar -xz -C /tmp; \
    mv "/tmp/linux-${arch}/helm" /usr/local/bin/helm; \
    rm -rf /tmp/linux-*; \
    wget -qO- "https://github.com/yannh/kubeconform/releases/download/${KUBECONFORM_VERSION}/kubeconform-linux-${arch}.tar.gz" \
      | tar -xz -C /usr/local/bin kubeconform; \
    helm version --short; \
    kubeconform -v

COPY --from=build /out/gitops-gate /usr/local/bin/gitops-gate

# The gate only ever reads.
RUN adduser -D -u 10001 gate
USER 10001

ENTRYPOINT ["/usr/local/bin/gitops-gate"]
CMD ["--help"]
