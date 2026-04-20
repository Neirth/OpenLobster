// Package version implements the "version" subcommand.
//
// # License
// See LICENSE in the root of the repository.
package version

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Command returns the cobra command for the "version" subcommand.
func Command(v string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			Run(cmd.OutOrStdout(), v)
		},
	}
}

// Run prints the build version to w.
func Run(w io.Writer, v string) {
	fmt.Fprintf(w, "openlobster %s\n", v)
}
