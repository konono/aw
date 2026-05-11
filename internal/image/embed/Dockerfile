FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates wget openssh-client sudo && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash claude && \
    echo 'claude ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Install mise as claude user
RUN su -s /bin/bash claude -c 'curl https://mise.jdx.dev/install.sh | sh'

ENV PATH="/home/claude/.local/bin:/home/claude/.local/share/mise/shims:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
# Create base directory for host path symlinks (owned by claude)
RUN mkdir -p /Users && chown claude:claude /Users

WORKDIR /workspace
# Note: WORKDIR is overridden at runtime via --workdir to match host path

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
