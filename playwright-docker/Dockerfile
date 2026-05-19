FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates wget openssh-client sudo xz-utils \
      # Playwright browser dependencies
      libglib2.0-0 libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
      libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 \
      libxfixes3 libxrandr2 libgbm1 libxcb1 libpango-1.0-0 \
      libcairo2 libasound2 libcups2 libdrm2 libxshmfence1 \
      libxkbcommon0 libatspi2.0-0 && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash agent && \
    echo 'agent ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Create /nix owned by agent for single-user Nix install
RUN mkdir -p /nix && chown agent:agent /nix

# Install Nix as agent user (single-user mode, no daemon)
USER agent
RUN curl -fsSL https://nixos.org/nix/install | sh -s -- --no-daemon
USER root

# Create default profile symlink that devbox expects
RUN ln -s /home/agent/.local/state/nix/profiles/profile /nix/var/nix/profiles/default

# Install devbox
RUN su -s /bin/bash agent -c 'curl -fsSL https://get.jetify.com/devbox | bash -s -- -f'

ENV HOME="/home/agent" \
    BASH_ENV="/home/agent/.aw_env.sh" \
    PATH="/home/agent/.nix-profile/bin:/home/agent/.local/bin:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /Users /workspace && chown agent:agent /Users /workspace

USER agent
WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
CMD ["bash"]
