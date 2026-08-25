FROM mcr.microsoft.com/devcontainers/go:1.26

RUN rm -f /etc/apt/sources.list.d/yarn.list

# ── Node / NVM ──────────────────────────────────────────────
ENV NVM_VERSION=0.40.3
ENV NODE_VERSION=24
ENV NVM_DIR=/root/.nvm

# ── System deps ─────────────────────────────────────────────
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    git \
    gnupg \
    wget \
    xvfb \
    --no-install-recommends

# ── Chrome ──────────────────────────────────────────────────
RUN dpkg --add-architecture amd64 \
    && apt-get update \
    && apt-get install -y \
        libasound2 \
        libatk-bridge2.0-0 \
        libatk1.0-0 \
        libatspi2.0-0 \
        libcairo2 \
        libcups2 \
        libcurl4 \
        libdbus-1-3 \
        libexpat1 \
        libgbm1 \
        libglib2.0-0 \
        libgtk-3-0 \
        libnspr4 \
        libnss3 \
        libpango-1.0-0 \
        libudev1 \
        libvulkan1 \
        libx11-6 \
        libxcb1 \
        libxcomposite1 \
        libxdamage1 \
        libxext6 \
        libxfixes3 \
        libxkbcommon0 \
        libxrandr2 \
        fonts-liberation \
        xdg-utils \
        --no-install-recommends
RUN wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
RUN dpkg -i google-chrome-stable_current_amd64.deb; apt-get install -fy
RUN rm google-chrome-stable_current_amd64.deb
RUN which google-chrome

ENV CHROME_BIN=/usr/bin/google-chrome-stable

SHELL ["/bin/bash", "-c"]

RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh | bash \
    && . "$NVM_DIR/nvm.sh" \
    && nvm install ${NODE_VERSION} \
    && nvm alias default ${NODE_VERSION} \
    && nvm use default \
    && ln -s $NVM_DIR/versions/node/$(nvm version ${NODE_VERSION}) $NVM_DIR/versions/node/v${NODE_VERSION}

ENV PATH=$NVM_DIR/versions/node/v${NODE_VERSION}/bin:$PATH