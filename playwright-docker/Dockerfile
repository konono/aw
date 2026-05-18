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

RUN su -s /bin/bash agent -c 'curl -fsSL https://get.jetify.com/devbox | bash -s -- -f'

ENV PATH="/home/agent/.nix-profile/bin:/home/agent/.local/bin:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /Users && chown agent:agent /Users

WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
CMD ["bash"]
