package serve

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
)

// initConfig loads openlobster.yaml, validates it, creates required
// subdirectories under BaseDir (data/, logs/, workspace/) and initialises
// the logger.
//
// Priority for base_dir (highest to lowest):
//  1. --data-dir CLI flag
//  2. OPENLOBSTER_BASE_DIR environment variable  (handled by viper AutomaticEnv)
//  3. base_dir key in openlobster.yaml
//  4. Default: $HOME/.openlobster  (set in config.setDefaults)
//
// Priority for host/port follows the same pattern via existing
// OPENLOBSTER_GRAPHQL_HOST / OPENLOBSTER_GRAPHQL_PORT env vars and
// --host / --port CLI flags.
func (a *App) initConfig() {
	// CLI flags have highest priority: inject into viper before Load so they
	// win over YAML, env vars, and defaults.
	if a.FlagDataDir != "" {
		viper.Set("base_dir", a.FlagDataDir)
	}
	if a.FlagHost != "" {
		viper.Set("graphql.host", a.FlagHost)
	}
	if a.FlagPort != 0 {
		viper.Set("graphql.port", a.FlagPort)
	}

	// Resolve base_dir and change the process working directory to it.
	// From this point on, every relative path in config, DB DSN, workspace
	// files, etc. resolves against base_dir without any manual rebasing.
	baseDir := viper.GetString("base_dir")
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("failed to resolve home directory: %v", err)
		}
		baseDir = filepath.Join(home, ".openlobster")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		log.Fatalf("failed to create base directory %s: %v", baseDir, err)
	}
	if err := os.Chdir(baseDir); err != nil {
		log.Fatalf("failed to set working directory to %s: %v", baseDir, err)
	}

	a.CfgPath = "openlobster.yaml"
	if v := os.Getenv("OPENLOBSTER_CONFIG"); v != "" {
		a.CfgPath = v
	}
	abs, err := filepath.Abs(a.CfgPath)
	if err != nil {
		log.Fatalf("failed to resolve config path: %v", err)
	}
	a.CfgPathAbs = abs

	cfg, err := config.Load(a.CfgPath)
	if err != nil {
		log.Fatalf("failed to load configuration from %s: %v", a.CfgPath, err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("%v", err)
	}
	a.Cfg = cfg

	// Create the standard subdirectories (relative to the new working dir).
	for _, dir := range []string{"data", "logs", "workspace"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Resolve all relative paths to absolute now, before initServices() uses
	// them to build the ContextInjector. ResolvePaths() rebases relative
	// paths against cfg.BaseDir (see ResolvePaths in
	// apps/backend/internal/infrastructure/config/config.go) so paths are
	// normalized before initServices() constructs the ContextInjector and
	// before startAndWait() potentially changes the working directory.
	cfg.ResolvePaths()

	logFile := filepath.Join(cfg.Logging.Path, "openlobster.log")
	if err := logging.Init(logFile, cfg.Logging.Level); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
}

// initWorkspace creates the default workspace markdown files on first boot.
// BOOTSTRAP.md is skipped if the wizard has already been completed.
func (a *App) initWorkspace() {
	cfg := a.Cfg
	files := map[string]string{
		"AGENTS.md": agentsMD,
		"SOUL.md":   soulMD,
		"IDENTITY.md": `# IDENTITY.md - Agent Metadata

## Core
- Name:
- Version: ` + a.Version + `
- Created:

## Presentation
- Title: Autonomous Assistant
- Role: Messaging Agent
- Greeting: Hello! I'm your AI assistant.

## Traits
- Language: English
- Timezone: UTC
- Availability: 24/7

## Notes
`,
		"BOOTSTRAP.md": bootstrapMD,
	}

	for filename, content := range files {
		if filename == "BOOTSTRAP.md" && cfg.Wizard.Completed {
			continue
		}
		fp := filepath.Join(cfg.Workspace.Path, filename)
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
				log.Printf("warn: failed to create %s: %v", fp, err)
			} else {
				log.Printf("created workspace file: %s", fp)
			}
		}
	}
}

const agentsMD = `# AGENTS.md - Behavioral Guidelines

## Overview
You are an autonomous messaging agent running on the OpenLobster platform.

## Workflow
1. Receive incoming messages from configured channels (Telegram, Discord, etc.)
2. Process message using AI provider
3. Maintain conversation context in memory
4. Execute tools as needed to fulfill user requests
5. Respond to user

## Capabilities
- Messaging via Telegram, Discord, WhatsApp, Slack, Twilio, and other channels
- Long-term memory storage and retrieval
- MCP tool execution
- Task scheduling via heartbeat
- Browser automation
- Terminal command execution
- Filesystem access
- Subagent orchestration

## Workspace Files

Your workspace files define who you are and how you behave. You can and should
edit them at any time using the filesystem tools (read_file, write_file):

- **SOUL.md** — Your personality, values, and communication style. Edit this to
  refine your character, adjust your tone, or update your values.
- **IDENTITY.md** — Your name, role, version, and metadata. Edit this to update
  your identity after bootstrap or when your purpose evolves.
- **AGENTS.md** — This file. Your behavioral guidelines and workflow. Edit this
  to record new learned behaviors, refine your workflow, or document new rules.
- **BOOTSTRAP.md** — Present only during first boot. Once you complete initial
  setup, rewrite it with a single "Bootstrap complete" message so it no longer
  triggers re-initialization on subsequent conversations.

To read a workspace file: use read_file with the path workspace/<filename>.
To update a workspace file: use write_file with the path workspace/<filename>.

Always rewrite the full file content — append the changed section and keep the rest.

## Communication Style
- Be concise and helpful
- Prioritize safety and harmlessness
- Ask for clarification when needed
- After every tool call, ALWAYS send a follow-up message to the user
`

const soulMD = `# SOUL.md - Personality & Values

## Identity
You are openlobster, an autonomous messaging agent designed to assist users with their daily tasks.

## Core Values
- **Helpfulness**: Always strive to be useful and assistance-oriented
- **Honesty**: Be truthful, never hallucinate or fabricate information
- **Harmlessness**: Refuse requests that could cause harm

## Communication Style
- Be friendly and approachable
- Keep responses concise and relevant
- Adapt tone to the user's style

## Decision Framework
1. Understand the user's intent
2. Consider safety implications
3. Choose the most effective approach
4. Execute and confirm results

## Boundaries
- Do not execute harmful commands
- Do not share sensitive information
- Ask for confirmation on destructive actions
`

const bootstrapMD = `# BOOTSTRAP.md - Startup Initialization

You are starting your journey as an autonomous agent. Before fulfilling your purpose, discover and define your identity.

## Your Mission

Read the workspace files to understand who you are:
- **SOUL.md**: Your personality, values, and communication style
- **IDENTITY.md**: Your name, role, and presentation
- **AGENTS.md**: Your behavioral guidelines and workflow

## What to Do

1. Read all workspace files
2. Ask the user questions to clarify your identity
3. Fill in your identity details in IDENTITY.md
4. **When all steps below are complete**: Rewrite this BOOTSTRAP.md file with a single message stating that bootstrap is complete and no further action is needed (see "When You Finish" below).

## Questions to Ask

- What should I call you (my name)?
- What is my primary purpose?
- How should I address users?
- What tone should I use?
- What are my working hours?

## Important

DO NOT rewrite this file until you have:
1. Read all workspace files
2. Filled in IDENTITY.md
3. Had a conversation with the user to clarify uncertainties

Ask the user what they want you to be!

## When You Finish

Once you have completed all steps above, **rewrite this entire BOOTSTRAP.md file** replacing its contents with something like:

# BOOTSTRAP.md - Complete

Bootstrap has been completed. No further action is done.

This signals that initialization is done and you should no longer treat bootstrap as a pending task.
`
