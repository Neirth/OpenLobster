//go:build !darwin && !linux

// # License
// See LICENSE in the root of the repository.
package daemon

import "fmt"

func install(binaryPath, openlobsterHome string) error {
	return fmt.Errorf("daemon management is not supported on this platform")
}

func uninstall() error {
	return fmt.Errorf("daemon management is not supported on this platform")
}

func status() error {
	return fmt.Errorf("daemon management is not supported on this platform")
}
