// Package migrate implements the "migrate" subcommand.
//
// It reads an OpenClaw configuration file and migrates the supported data
// to a running OpenLobster instance via its GraphQL API, and copies the
// OpenClaw workspace directory to the OpenLobster workspace.
//
// Migrated data:
//   - Agent and channel configuration (via updateConfig mutation)
//   - Scheduled tasks / cron jobs (via addTask mutation)
//   - Workspace directory (direct filesystem copy)
//
// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultEndpoint = "http://localhost:8080/graphql"
	apiKeyEnvVar    = "OPENLOBSTER_API_KEY"
)

// Command returns the cobra command for the "migrate" subcommand.
func Command() *cobra.Command {
	var (
		src      string
		dst      string
		endpoint string
		apiKey   string
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate an OpenClaw installation to OpenLobster via GraphQL",
		Long: `Reads an OpenClaw home directory and migrates the supported data
to a running OpenLobster instance via its GraphQL API.

Migrated:
  - Agent and channel configuration (via updateConfig)
  - Scheduled tasks / cron jobs (via addTask)
  - Workspace directory (filesystem copy)

Note: WhatsApp credentials cannot be migrated — OpenClaw uses Baileys (QR)
while OpenLobster requires a Meta Cloud API phone_id and api_token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiKey == "" {
				apiKey = os.Getenv(apiKeyEnvVar)
			}

			srcAbs, err := filepath.Abs(expandHome(src))
			if err != nil {
				return fmt.Errorf("cannot resolve --from path: %w", err)
			}

			dstAbs, err := filepath.Abs(expandHome(dst))
			if err != nil {
				return fmt.Errorf("cannot resolve --to path: %w", err)
			}

			cfg := readOpenClaw(srcAbs)
			enrichWithEnv(cfg, srcAbs)
			client := newGQLClient(endpoint, apiKey, dryRun)
			workspaceSrc := resolveWorkspaceSrc(cfg, srcAbs)
			workspaceDst := filepath.Join(dstAbs, "workspace")

			return runMigration(cfg, client, workspaceSrc, workspaceDst, dryRun)
		},
	}

	cmd.Flags().StringVar(&src, "from", defaultOpenClawHome(), "path to OpenClaw home directory")
	cmd.Flags().StringVar(&dst, "to", defaultOpenLobsterHome(), "path to OpenLobster home directory")
	cmd.Flags().StringVar(&endpoint, "endpoint", defaultEndpoint, "OpenLobster GraphQL endpoint")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "OpenLobster API key (or set "+apiKeyEnvVar+" env var)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be migrated without making API calls")

	return cmd
}

// runMigration orchestrates all migration steps in order.
func runMigration(cfg viperReader, c *gqlClient, workspaceSrc, workspaceDst string, dryRun bool) error {
	var failed []string

	if err := migrateConfig(cfg, c); err != nil {
		failed = append(failed, fmt.Sprintf("config: %v", err))
	}

	if err := migrateTasks(cfg, c); err != nil {
		failed = append(failed, fmt.Sprintf("tasks: %v", err))
	}

	if err := migrateWorkspace(workspaceSrc, workspaceDst, dryRun); err != nil {
		failed = append(failed, fmt.Sprintf("workspace: %v", err))
	}

	printWhatsAppWarning()

	if len(failed) > 0 {
		return fmt.Errorf("migration completed with errors:\n  %s", strings.Join(failed, "\n  "))
	}
	return nil
}

func printWhatsAppWarning() {
	fmt.Println(`
Note: WhatsApp credentials were NOT migrated.
  OpenClaw uses Baileys (QR-code / WhatsApp Web); OpenLobster uses the
  Meta Cloud API and requires a phone_id and api_token from
  developers.facebook.com. Set them manually:

    openlobster config set \
      channels.whatsapp.enabled true \
      channels.whatsapp.phone_id  <YOUR_PHONE_ID> \
      channels.whatsapp.api_token <YOUR_API_TOKEN>`)
}
