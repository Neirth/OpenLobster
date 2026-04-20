// Package serve — cobra command entry point.
//
// # License
// See LICENSE in the root of the repository.
package serve

import (
	"io/fs"

	"github.com/spf13/cobra"
)

// Command returns the cobra command for the "serve" subcommand.
func Command(version string, publicFS fs.FS) *cobra.Command {
	var host string
	var port int
	var dataDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server and all messaging adapters (default)",
		Run: func(cmd *cobra.Command, args []string) {
			app := New(version, publicFS)
			if cmd.Flags().Changed("host") {
				app.FlagHost = host
			}
			if cmd.Flags().Changed("port") {
				app.FlagPort = port
			}
			if cmd.Flags().Changed("data-dir") {
				app.FlagDataDir = dataDir
			}
			app.Run()
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "IP address to bind (overrides config and OPENLOBSTER_HOST)")
	cmd.Flags().IntVar(&port, "port", 0, "TCP port to listen on (overrides config and OPENLOBSTER_PORT)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Base data directory (overrides config and OPENLOBSTER_DATA_DIR)")

	return cmd
}
