FROM alpine:3.21

ARG KUBECTL_VERSION=v1.34.1
ARG TARGETARCH

RUN apk add --no-cache \
    bash \
    curl \
    ca-certificates \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/ \
    && kubectl version --client

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/kubectl"]
