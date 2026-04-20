// openlobster — autonomous messaging agent daemon.
//
// Usage: openlobster [command] [flags]
//
// Commands:
//
//		config   Read or write configuration keys in the YAML (respects encryption)
//	 daemon   Tool for install daemon services in the user folder
//		migrate  Migrate an OpenClaw config file to OpenLobster format
//		serve    Start the HTTP server and all messaging adapters (default)
//		version  Print build version and exit
//
// # License
// See LICENSE in the root of the repository.
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cmdconfig "github.com/neirth/openlobster/cmd/openlobster/config"
	cmddaemon "github.com/neirth/openlobster/cmd/openlobster/daemon"
	cmdmigrate "github.com/neirth/openlobster/cmd/openlobster/migrate"
	cmdserve "github.com/neirth/openlobster/cmd/openlobster/serve"
	cmdversion "github.com/neirth/openlobster/cmd/openlobster/version"
)

// version is set at build time via -ldflags "-X main.version=x.y.z"
var version = "dev"

// public is the embedded frontend + static assets.
//
//go:embed all:public
var public embed.FS

func main() {
	// Disable Ollama SDK key-based auth; we use Bearer token via our own transport.
	if os.Getenv("OLLAMA_AUTH") == "" {
		os.Setenv("OLLAMA_AUTH", "false")
	}

	root := &cobra.Command{
		Use:           "openlobster",
		Short:         "Autonomous messaging agent daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Running "openlobster" with no subcommand starts the server.
		Run: func(cmd *cobra.Command, args []string) {
			cmdserve.New(version, public).Run()
		},
	}

	root.AddCommand(
		cmdconfig.Command(),
		cmddaemon.Command(),
		cmdmigrate.Command(),
		cmdserve.Command(version, public),
		cmdversion.Command(version),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
