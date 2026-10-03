package main

import "runtime/debug"

// version is what --version reports. A release sets it with the linker, from
// the tag being built: -ldflags "-X main.version=<version>".
var version = "dev"

func buildVersion() string {
	if version != "dev" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}
