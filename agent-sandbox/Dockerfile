FROM node:22-bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ripgrep ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

ARG CODEX_VERSION=latest
RUN npm install --global "@openai/codex@${CODEX_VERSION}"

ARG CLAUDE_CODE_VERSION=latest
RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"

ARG OPENCODE_VERSION=latest
RUN npm install --global "opencode-ai@${OPENCODE_VERSION}"

ARG PI_VERSION=latest
RUN npm install --global --ignore-scripts "@earendil-works/pi-coding-agent@${PI_VERSION}"

RUN mkdir -p /agent-sandbox-images

WORKDIR /workspace
CMD ["bash"]
