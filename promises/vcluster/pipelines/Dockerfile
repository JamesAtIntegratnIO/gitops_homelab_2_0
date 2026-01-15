FROM alpine:3.19

# Install required tools
RUN apk add --no-cache bash yq curl kubectl

# Copy pipeline scripts
COPY configure-pipeline.sh /scripts/configure-pipeline.sh
COPY delete-pipeline.sh /scripts/delete-pipeline.sh

# Make scripts executable
RUN chmod +x /scripts/*.sh

# Default to configure pipeline
CMD ["/scripts/configure-pipeline.sh"]
