# Images and networking

## Provide your project's toolchain

The default image contains the agents and their supporting tools. If the
task needs a language runtime, choose a compatible base image that includes it:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine \
  -q "run gofmt and go test ./..., then fix failures"
```

`-i` is short for `--baseImage`. The tool builds a local image derived from
your base, adding Node.js, npm, Bash, the agents, ripgrep, CA certificates,
curl, Git, Python 3 (also available as `python`), and Poppler PDF tools
(`pdftotext` and `pdfinfo`). These tools are also included in default image
builds. Later invocations reuse the image and check the selected agent's
version against npm, rebuilding when it differs.

Supported bases are Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9, and
Amazon Linux 2023. They must provide `apk`, `apt-get`, `dnf`, or `microdnf`;
`yum` and other package managers are not supported. Existing Node.js and npm
are reused when both are available. Otherwise, Alpine installs its Node.js
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

## Mount reference files

Use repeatable `--file` flags to provide PDFs, CSVs, or other regular files
to any of the four agents:

```bash
agent-sandbox run -a claude \
  --file "/path/to/spec.pdf" \
  --file "/path/to/sample.csv" \
  -q 'read "spec.pdf" and "sample.csv", then implement the requested feature'
```

Each path must be an existing regular file. Relative paths resolve from the
directory where you invoke the command, including paths in JSON configuration.
Files are mounted read-only under `/agent-sandbox-files/<number>/<filename>`.
The prompt lists filenames and container paths; duplicate filenames have
separate numbered directories, so refer to their container paths when needed.

`--file` works with `run`, `resume`, `run-tui`, and `resume-tui`. You can also
set a [JSON `file` array](configuration.md#share-reference-files). It supplies
reference material; `-f` / `--filePrompt` reads the task instructions themselves.

Ask the agent to read or extract the files as part of your task. Files are not
automatically converted. Poppler tools are installed when images are built;
there is no runtime check or automatic rebuild for missing PDF tools.

## Reach services on your host

When a task needs access to local services, enable host networking explicitly:

```bash
agent-sandbox run -a codex --hn \
  -q "inspect the API running on localhost:8080 and update the client"
```

`--hn` shares the host network with the container, giving access to local
services without a port allowlist. It reduces network isolation and is disabled
by default. Use `--hn=false` to override an enabled JSON setting.
