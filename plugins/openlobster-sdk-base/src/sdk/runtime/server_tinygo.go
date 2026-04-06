//go:build tinygo

// Copyright (c) OpenLobster contributors. See LICENSE for details.

package runtime

import "fmt"

// Run in tinygo builds is compile-only for SDK API compatibility.
// The native OpenLobster host runtime currently requires the non-tinygo gRPC server path.
func Run(plugin Plugin) error {
	if plugin.Exports == nil || len(plugin.Exports) == 0 {
		return fmt.Errorf("runtime: tinygo compile path requires at least one export")
	}
	return fmt.Errorf("runtime: tinygo build is compile-only; native host runtime requires non-tinygo build")
}

// MustRun keeps source compatibility for plugin mains under tinygo builds.
func MustRun(plugin Plugin) {
	if err := Run(plugin); err != nil {
		panic(err)
	}
}
