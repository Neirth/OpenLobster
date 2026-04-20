// Package daemon implements the "daemon" subcommand, which installs, uninstalls,
// and reports the status of the OpenLobster user-space daemon.
//
// On macOS the daemon is managed via launchd (~/Library/LaunchAgents/).
// On Linux it is managed via systemd --user (~/.config/systemd/user/).
//
// # License
// See LICENSE in the root of the repository.
package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Command returns the cobra command tree for the "daemon" subcommand.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the OpenLobster user-space background daemon",
		Long: `Install, uninstall, or check the status of the OpenLobster daemon.

The daemon runs as a user-space service — no root privileges are required.

  macOS  — launchd,          ~/Library/LaunchAgents/com.openlobster.plist
  Linux  — systemd --user,   ~/.config/systemd/user/openlobster.service`,
	}

	cmd.AddCommand(installCmd(), uninstallCmd(), statusCmd())
	return cmd
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			bin, err := resolvedExecutable()
			if err != nil {
				return fmt.Errorf("cannot determine binary path: %w", err)
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home directory: %w", err)
			}
			return install(bin, filepath.Join(home, ".openlobster"))
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstall()
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return status()
		},
	}
}

// resolvedExecutable returns the path of the current binary, following
// symlinks so the service manager always gets the real file path.
func resolvedExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}
