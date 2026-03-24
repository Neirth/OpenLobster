// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Shared GraphQL query strings used by both the web frontend and the terminal.
 *
 * Both apps/frontend and apps/terminal query the same GraphQL server.
 * Keeping these strings here eliminates duplication and ensures both surfaces
 * stay in sync when the schema changes.
 *
 * Import with:
 *   import { METRICS_QUERY } from '@openlobster/ui/graphql/queries';
 */

// ─── Agent ────────────────────────────────────────────────────────────────────

export const AGENT_QUERY = /* GraphQL */ `
  query GetAgent {
    agent {
      id
      name
      version
      status
      uptime
      provider
    }
  }
`;

// ─── Metrics ──────────────────────────────────────────────────────────────────

export const METRICS_QUERY = /* GraphQL */ `
  query GetMetrics {
    metrics {
      uptime
      messagesReceived
      messagesSent
      activeSessions
      memoryNodes
      memoryEdges
      mcpTools
      tasksPending
      tasksRunning
      tasksDone
      errorsTotal
    }
  }
`;

// ─── Channels ─────────────────────────────────────────────────────────────────

export const CHANNELS_QUERY = /* GraphQL */ `
  query GetChannels {
    channels {
      id
      name
      type
      status
      messagesReceived
      messagesSent
    }
  }
`;

// ─── Conversations ─────────────────────────────────────────────────────────────

export const CONVERSATIONS_QUERY = /* GraphQL */ `
  query GetConversations {
    conversations {
      id
      channelId
      channelName
      groupName
      isGroup
      participantId
      participantName
      lastMessageAt
      unreadCount
    }
  }
`;

export const MESSAGES_QUERY = /* GraphQL */ `
  query GetMessages($conversationId: String!, $before: String, $limit: Int) {
    messages(conversationId: $conversationId, before: $before, limit: $limit) {
      id
      conversationId
      role
      content
      createdAt
      attachments {
        type
        url
        filename
        mimeType
      }
    }
  }
`;

// ─── Tasks (Cron) ──────────────────────────────────────────────────────────────

export const TASKS_QUERY = /* GraphQL */ `
  query GetTasks {
    tasks {
      id
      prompt
      status
      schedule
      taskType
      isCyclic
      enabled
      createdAt
      lastRunAt
      nextRunAt
    }
  }
`;

// ─── MCP Servers ───────────────────────────────────────────────────────────────

export const MCP_SERVERS_QUERY = /* GraphQL */ `
  query GetMcpServers {
    mcpServers {
      name
      transport
      status
      toolCount
      url
    }
  }
`;

export const MCP_TOOLS_QUERY = /* GraphQL */ `
  query GetMcpTools {
    mcpTools {
      name
      serverName
      description
    }
  }
`;

// ─── Tool Permissions ──────────────────────────────────────────────────────────

export const MCP_USERS_QUERY = /* GraphQL */ `
  query GetMcpUsers {
    mcpUsers {
      channelId
      displayName
      isAgent
    }
  }
`;

export const TOOL_PERMISSIONS_QUERY = /* GraphQL */ `
  query GetToolPermissions($userId: String!) {
    toolPermissions(userId: $userId) {
      toolName
      mode
    }
  }
`;

// ─── Memory ────────────────────────────────────────────────────────────────────

export const MEMORY_QUERY = /* GraphQL */ `
  query GetMemory {
    memory {
      nodes {
        id
        label
        type
        value
        createdAt
        properties
      }
      edges {
        id
        sourceId
        targetId
        relation
      }
    }
  }
`;

// ─── Skills ────────────────────────────────────────────────────────────────────

export const SKILLS_QUERY = /* GraphQL */ `
  query GetSkills {
    skills {
      name
      description
      enabled
      path
    }
  }
`;

// ─── Configuration ─────────────────────────────────────────────────────────────

export const CONFIG_QUERY = /* GraphQL */ `
  query GetConfig {
    config {
      agent {
        name
        systemPrompt
        provider
        model
        apiKey
        baseURL
        ollamaHost
        ollamaApiKey
        anthropicApiKey
        dockerModelRunnerEndpoint
        dockerModelRunnerModel
        reasoningLevel
      }
      capabilities {
        browser
        terminal
        subagents
        memory
        mcp
        filesystem
        sessions
      }
      database {
        driver
        dsn
        maxOpenConns
        maxIdleConns
      }
      memory {
        backend
        filePath
        neo4j {
          uri
          user
          password
        }
      }
      subagents {
        maxConcurrent
        defaultTimeout
      }
      graphql {
        enabled
        port
        host
        baseUrl
      }
      logging {
        level
        path
      }
      secrets {
        backend
        file {
          path
        }
        openbao {
          url
          token
        }
      }
      scheduler {
        enabled
        memoryEnabled
        memoryInterval
      }
      activeSessions {
        id
        address
        status
        channel
        user
      }
      channels {
        channelId
        channelName
        enabled
      }
      channelSecrets {
        telegramEnabled
        telegramToken
        discordEnabled
        discordToken
        whatsAppEnabled
        whatsAppPhoneId
        whatsAppApiToken
        twilioEnabled
        twilioAccountSid
        twilioAuthToken
        twilioFromNumber
        slackEnabled
        slackBotToken
        slackAppToken
      }
      wizardCompleted
    }
  }
`;

// ─── System Files ─────────────────────────────────────────────────────────────

export const SYSTEM_FILES_QUERY = /* GraphQL */ `
  query GetSystemFiles {
    systemFiles {
      name
      content
    }
  }
`;
