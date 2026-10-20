ARG BASE_IMAGE=node:22-bookworm-slim
FROM ${BASE_IMAGE} AS browser-tools

USER 0
RUN set -eu; \
    if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache bash ripgrep ca-certificates curl git poppler-utils python3 chromium font-noto; \
        if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
            apk add --no-cache nodejs npm; \
        fi; \
    elif command -v apt-get >/dev/null 2>&1; then \
        apt-get update; \
        apt-get install -y --no-install-recommends \
            bash ca-certificates curl git poppler-utils ripgrep python3 chromium fonts-liberation; \
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
        "$rpm_pm" install -y bash ca-certificates curl git poppler-utils ripgrep python3 chromium liberation-sans-fonts; \
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
    fi; \
    if ! command -v python >/dev/null 2>&1; then \
        mkdir -p /usr/local/bin; \
        ln -s "$(command -v python3)" /usr/local/bin/python; \
    fi
RUN set -eu; \
    printf '\n# Keep the base image toolchain available to login shells.\nPATH=%s\nexport PATH\n' "$PATH" >> /etc/profile

# Keep browser automation outside the target project and every credential home.
# Distribution Chromium also works on musl-based Alpine without a second download.
ARG PUPPETEER_VERSION=25.15.0
RUN npm install --prefix /opt/agent-sandbox-browser --omit=dev --ignore-scripts --engine-strict \
    "puppeteer-core@${PUPPETEER_VERSION}"
RUN set -eu; \
    browser_binary="$(command -v chromium || command -v chromium-browser)"; \
    ln -s "$browser_binary" /usr/local/bin/agent-sandbox-chromium; \
    printf '%s\n' \
      'const fs = require("node:fs");' \
      'const path = require("node:path");' \
      'const os = require("node:os");' \
      'const executablePath = "/usr/local/bin/agent-sandbox-chromium";' \
      'exports.launch = async function () {' \
      '  const {default: puppeteer} = await import("puppeteer-core");' \
      '  const home = fs.mkdtempSync(path.join(os.tmpdir(), "agent-sandbox-browser-"));' \
      '  try {' \
      '    const browser = await puppeteer.launch({' \
      '      executablePath, headless: true,' \
      '      env: {...process.env, HOME: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home},' \
      '      args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],' \
      '    });' \
      '    browser.once("disconnected", () => fs.rmSync(home, {recursive: true, force: true}));' \
      '    return browser;' \
      '  } catch (error) {' \
      '    fs.rmSync(home, {recursive: true, force: true});' \
      '    throw error;' \
      '  }' \
      '};' \
      'async function main() {' \
      '  const [url, output, selector, ...extra] = process.argv.slice(2);' \
      '  if (!url || !output || extra.length) {' \
      '    throw new Error("Usage: agent-sandbox-browser <url> <output.png> [required-selector]");' \
      '  }' \
      '  const browser = await exports.launch();' \
      '  try {' \
      '    const page = await browser.newPage();' \
      '    await page.setViewport({width: 1280, height: 800});' \
      '    const response = await page.goto(url, {waitUntil: "networkidle2", timeout: 30000});' \
      '    if (response && !response.ok()) throw new Error(`HTTP ${response.status()}`);' \
      '    if (selector) await page.waitForSelector(selector, {visible: true, timeout: 10000});' \
      '    await page.screenshot({path: output, type: "png", fullPage: true});' \
      '    console.log(`Screenshot saved: ${output}`);' \
      '  } finally {' \
      '    await browser.close();' \
      '  }' \
      '}' \
      'if (require.main === module) main().catch(error => {console.error(error.message); process.exitCode = 1;});' \
      > /opt/agent-sandbox-browser/browser.cjs; \
    printf '%s\n' '#!/bin/sh' 'exec node /opt/agent-sandbox-browser/browser.cjs "$@"' \
      > /usr/local/bin/agent-sandbox-browser; \
    chmod 755 /usr/local/bin/agent-sandbox-browser; \
    node --check /opt/agent-sandbox-browser/browser.cjs

FROM browser-tools

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
