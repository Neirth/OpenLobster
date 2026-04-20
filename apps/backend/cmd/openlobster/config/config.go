// Package config implements the "config" subcommand.
//
// Usage:
//
//	openlobster config get <key> [<key> ...]
//	openlobster config set <key> <value> [<key> <value> ...]
//
// The config file path is resolved from $OPENLOBSTER_CONFIG or the default
// data/openlobster.yaml. Both encrypted (OLENC1 prefix) and plain YAML files
// are supported transparently.
//
// # License
// See LICENSE in the root of the repository.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Command returns the cobra command tree for the "config" subcommand.
func Command() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read or write configuration keys (encryption-aware)",
		Long: `Read or write keys in the OpenLobster YAML config file.
Both plain and encrypted (OLENC1 prefix) files are supported transparently.`,
	}

	cmd.PersistentFlags().StringVar(&cfgPath, "config", defaultCfgPath(), "path to openlobster.yaml")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "get <key> [<key> ...]",
			Short: "Print one or more config values",
			Args:  cobra.MinimumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				abs, err := filepath.Abs(cfgPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "config: cannot resolve path %q: %v\n", cfgPath, err)
					os.Exit(1)
				}
				runGet(abs, args)
			},
		},
		&cobra.Command{
			Use:   "set <key> <value> [<key> <value> ...]",
			Short: "Write one or more config values",
			Args:  cobra.MinimumNArgs(2),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args)%2 != 0 {
					fmt.Fprintln(os.Stderr, "config set: arguments must be <key> <value> pairs")
					os.Exit(1)
				}
				abs, err := filepath.Abs(cfgPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "config: cannot resolve path %q: %v\n", cfgPath, err)
					os.Exit(1)
				}
				runSet(abs, args)
			},
		},
	)

	return cmd
}

// runGet prints the value of each requested key.
func runGet(cfgPath string, keys []string) {
	v := loadViper(cfgPath)
	for _, key := range keys {
		val := v.Get(key)
		if val == nil {
			fmt.Printf("%s = (not set)\n", key)
		} else {
			fmt.Printf("%s = %v\n", key, val)
		}
	}
}

// runSet writes key=value pairs to the config file.
// Values are cast to the appropriate type (bool, int, float, string) by Viper.
func runSet(cfgPath string, pairs []string) {
	v := loadViper(cfgPath)
	for i := 0; i < len(pairs); i += 2 {
		key, val := pairs[i], pairs[i+1]
		v.Set(key, val)
		fmt.Printf("  %s = %s\n", key, val)
	}

	if err := config.WriteEncryptedConfigFromSettings(v.AllSettings(), cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "config set: failed to write %s: %v\n", cfgPath, err)
		os.Exit(1)
	}
	fmt.Printf("saved → %s\n", cfgPath)
}

// loadViper reads cfgPath (decrypting if needed) into a fresh Viper instance.
func loadViper(cfgPath string) *viper.Viper {
	data, err := config.ReadConfigBytes(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot read %s: %v\n", cfgPath, err)
		os.Exit(1)
	}
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot parse %s: %v\n", cfgPath, err)
		os.Exit(1)
	}
	return v
}

func defaultCfgPath() string {
	if v := os.Getenv("OPENLOBSTER_CONFIG"); v != "" {
		return v
	}
	return "data/openlobster.yaml"
}
