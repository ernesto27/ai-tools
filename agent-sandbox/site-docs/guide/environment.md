# Images and networking

## Provide your project's toolchain

The default image contains the agents and their supporting tools. If the
task needs a language runtime, choose a compatible base image that includes it:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine \
  -q "run gofmt and go test ./..., then fix failures"
```

`-i` is short for `--base-image`. The tool builds a local image derived from
your base, adding Node.js, npm, Bash, the agents, ripgrep, CA certificates,
curl, and Git. Later invocations reuse it and check the selected agent's
version against npm, rebuilding when it differs.

Supported bases are Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9, and
Amazon Linux 2023. They must provide `apk`, `apt-get`, `dnf`, or `microdnf`;
`yum` and other package managers are not supported. Alpine uses its Node.js
packages; other supported bases use Node.js 22 from NodeSource.

RHEL bases need working repositories and any required subscription within
the image. The tool does not mount host subscription credentials or enable
EPEL or CRB/CodeReady Builder. Missing packages cause the build to fail.

To inspect derived images:

```bash
docker image ls 'agent-sandbox-base-*'
```

Remove an unused derived image with `docker image rm <IMAGE_ID>` when you
want to reclaim space. It will be rebuilt if needed again.

## Attach screenshots or reference images

Codex and Claude Code accept image attachments. Pass an existing regular file
on the host for each `--image`:

```bash
agent-sandbox run -a claude \
  --image /path/to/mockup.png \
  --image /path/to/reference.png \
  -q "compare these screenshots and implement the resulting UI"
```

Attachments are mounted read-only. opencode and pi ignore them.
`--image` selects an attachment; `-i` selects a Docker base image.

## Reach services on your host

When a task needs access to local services, enable host networking explicitly:

```bash
agent-sandbox run -a codex --hn \
  -q "inspect the API running on localhost:8080 and update the client"
```

`--hn` shares the host network with the container, giving access to local
services without a port allowlist. It reduces network isolation and is disabled
by default. Use `--hn=false` to override an enabled JSON setting.
