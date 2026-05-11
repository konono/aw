FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates wget openssh-client sudo \
      # Playwright browser dependencies
      libglib2.0-0 libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
      libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 \
      libxfixes3 libxrandr2 libgbm1 libxcb1 libpango-1.0-0 \
      libcairo2 libasound2 libcups2 libdrm2 libxshmfence1 \
      libxkbcommon0 libatspi2.0-0 && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash claude && \
    echo 'claude ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

RUN su -s /bin/bash claude -c 'curl https://mise.jdx.dev/install.sh | sh'

ENV PATH="/home/claude/.local/bin:/home/claude/.local/share/mise/shims:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /Users && chown claude:claude /Users

WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
