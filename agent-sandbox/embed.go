// Package agentsandbox embeds the build context of the sandbox image so the
// compiled binary can build it from anywhere, without carrying the Dockerfile
// alongside.
package agentsandbox

import _ "embed"

// Dockerfile is the sandbox image definition, kept next to this file so it stays
// editable as a plain Dockerfile.
//
//go:embed Dockerfile
var Dockerfile string

// InstallScript is embedded so updates use the same installer as a fresh install
// without requiring a repository checkout.
//
//go:embed install.sh
var InstallScript string
