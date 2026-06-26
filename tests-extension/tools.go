//go:build tools
// +build tools

// This file ensures tool dependencies are tracked in go.mod and vendored
package tools

import (
	_ "github.com/go-bindata/go-bindata/v3/go-bindata"
)
