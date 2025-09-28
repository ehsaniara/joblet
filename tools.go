//go:build tools

package main

// This file ensures that build-time dependencies are tracked in go.mod
// even though they're not imported in regular Go code.
// The proto generation uses the version specified in go.mod.

import (
	_ "github.com/ehsaniara/joblet-proto"
)
