ARG BASE_IMAGE=node:22-bookworm-slim
FROM ${BASE_IMAGE}

USER 0
RUN set -eu; \
    if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache bash ripgrep ca-certificates curl git poppler-utils; \
        if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
            apk add --no-cache nodejs npm; \
        fi; \
    elif command -v apt-get >/dev/null 2>&1; then \
        apt-get update; \
        apt-get install -y --no-install-recommends \
            bash ca-certificates curl git poppler-utils ripgrep; \
        if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
            apt-get install -y --no-install-recommends gnupg; \
            mkdir -p /etc/apt/keyrings; \
            curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key \
                | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg; \
            echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" \
                > /etc/apt/sources.list.d/nodesource.list; \
            apt-get update; \
            apt-get install -y --no-install-recommends nodejs; \
        fi; \
        rm -rf /var/lib/apt/lists/*; \
    elif command -v dnf >/dev/null 2>&1 \
        || command -v microdnf >/dev/null 2>&1; then \
        if command -v dnf >/dev/null 2>&1; then rpm_pm=dnf; \
        else rpm_pm=microdnf; fi; \
        "$rpm_pm" install -y bash ca-certificates curl git poppler-utils ripgrep; \
        if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
            rpm_arch="$(uname -m)"; \
            case "$rpm_arch" in \
                x86_64|aarch64) ;; \
                *) echo "unsupported RPM architecture: $rpm_arch" >&2; exit 1 ;; \
            esac; \
            mkdir -p /etc/yum.repos.d; \
            printf '%s\n' \
                '[nodesource-nodejs]' \
                "name=Node.js Packages for Linux RPM based distros - $rpm_arch" \
                "baseurl=https://rpm.nodesource.com/pub_22.x/nodistro/nodejs/$rpm_arch" \
                'priority=9' \
                'enabled=1' \
                'gpgcheck=1' \
                'gpgkey=https://rpm.nodesource.com/gpgkey/ns-operations-public.key' \
                'module_hotfixes=1' \
                > /etc/yum.repos.d/nodesource-nodejs.repo; \
            "$rpm_pm" install -y nodejs; \
        fi; \
        "$rpm_pm" clean all; \
    else \
        echo "unsupported base image: apk, apt-get, dnf, or microdnf is required" >&2; exit 1; \
    fi
RUN set -eu; \
    printf '\n# Keep the base image toolchain available to login shells.\nPATH=%s\nexport PATH\n' "$PATH" >> /etc/profile

ARG CODEX_VERSION=latest
RUN npm install --global "@openai/codex@${CODEX_VERSION}"

ARG CLAUDE_CODE_VERSION=latest
RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"

ARG OPENCODE_VERSION=latest
RUN npm install --global "opencode-ai@${OPENCODE_VERSION}"

ARG PI_VERSION=latest
RUN npm install --global --ignore-scripts "@earendil-works/pi-coding-agent@${PI_VERSION}"

RUN mkdir -p /agent-sandbox-images /agent-sandbox-files

WORKDIR /workspace
CMD ["bash"]
