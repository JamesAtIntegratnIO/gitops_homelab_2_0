FROM alpine:3.19

RUN apk add --no-cache bash yq curl jq

COPY pipelines/configure-pipeline.sh /scripts/configure-pipeline.sh
COPY pipelines/delete-pipeline.sh /scripts/delete-pipeline.sh
COPY internal/ /workdir/internal/

RUN chmod +x /scripts/*.sh /workdir/internal/configure-pipeline/scripts/*.sh /workdir/internal/delete-pipeline/scripts/*.sh

CMD ["/scripts/configure-pipeline.sh"]
