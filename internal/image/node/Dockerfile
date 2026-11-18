FROM docker:27-dind

# Install basic tools
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    iptables \
    ip6tables \
    iproute2 \
    net-tools \
    ca-certificates \
    shadow

# Create torresmo data directory
RUN mkdir -p /var/lib/torresmo

# Copy the entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose Docker daemon socket
VOLUME /var/lib/docker

# The DinD image already sets up dockerd as entrypoint
# We wrap it to also start our agent
ENTRYPOINT ["entrypoint.sh"]
