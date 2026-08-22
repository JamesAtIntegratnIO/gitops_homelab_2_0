# The gate shells out to helm and kubeconform rather than vendoring their
# libraries: chart rendering has to match what the cluster's own Helm does, and
# pinning a library version is a slower way to drift away from that.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/gitops-gate .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git bash

ARG HELM_VERSION=v3.19.0
ARG KUBECONFORM_VERSION=v0.7.0
# No default. TARGETARCH is a built-in BuildKit arg, and assigning one here
# shadows the value BuildKit injects -- which made the arm64 image download
# amd64 helm and kubeconform, then fail exec'ing them under emulation.
ARG TARGETARCH

RUN set -eux; \
    wget -qO- "https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" \
      | tar -xz -C /tmp; \
    mv "/tmp/linux-${TARGETARCH}/helm" /usr/local/bin/helm; \
    rm -rf /tmp/linux-*; \
    wget -qO- "https://github.com/yannh/kubeconform/releases/download/${KUBECONFORM_VERSION}/kubeconform-linux-${TARGETARCH}.tar.gz" \
      | tar -xz -C /usr/local/bin kubeconform; \
    helm version --short; \
    kubeconform -v

COPY --from=build /out/gitops-gate /usr/local/bin/gitops-gate

# The gate only ever reads.
RUN adduser -D -u 10001 gate
USER 10001

ENTRYPOINT ["/usr/local/bin/gitops-gate"]
CMD ["--help"]
