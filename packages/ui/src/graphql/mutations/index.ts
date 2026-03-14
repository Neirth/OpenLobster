// Copyright (c) OpenLobster contributors. See LICENSE for details.

/**
 * Shared GraphQL mutation strings used by both the web frontend and the terminal.
 *
 * Import with:
 *   import { SEND_MESSAGE_MUTATION } from '@openlobster/ui/graphql/mutations';
 */

// ─── Chat ──────────────────────────────────────────────────────────────────────

export const SEND_MESSAGE_MUTATION = /* GraphQL */ `
  mutation SendMessage($conversationId: String!, $content: String!) {
    sendMessage(conversationId: $conversationId, content: $content) {
      id
      conversationId
      role
      content
      createdAt
    }
  }
`;

// ─── Tasks (Cron) ──────────────────────────────────────────────────────────────

export const ADD_TASK_MUTATION = /* GraphQL */ `
  mutation AddTask($prompt: String!, $schedule: String) {
    addTask(prompt: $prompt, schedule: $schedule) {
      id
      prompt
      status
      schedule
      taskType
      isCyclic
      createdAt
      lastRunAt
      nextRunAt
    }
  }
`;

export const COMPLETE_TASK_MUTATION = /* GraphQL */ `
  mutation CompleteTask($taskId: String!) {
    completeTask(taskId: $taskId)
  }
`;

export const REMOVE_TASK_MUTATION = /* GraphQL */ `
  mutation RemoveTask($taskId: String!) {
    removeTask(taskId: $taskId)
  }
`;

export const TOGGLE_TASK_MUTATION = /* GraphQL */ `
  mutation ToggleTask($id: String!, $enabled: Boolean!) {
    toggleTask(id: $id, enabled: $enabled) {
      success
      id
      enabled
    }
  }
`;

export const UPDATE_TASK_MUTATION = /* GraphQL */ `
  mutation UpdateTask($id: String!, $prompt: String!, $schedule: String) {
    updateTask(id: $id, prompt: $prompt, schedule: $schedule) {
      id
      prompt
      status
      schedule
      taskType
      isCyclic
      createdAt
      lastRunAt
      nextRunAt
    }
  }
`;

// ─── MCP Servers ───────────────────────────────────────────────────────────────

export const CONNECT_MCP_MUTATION = /* GraphQL */ `
  mutation ConnectMcp($name: String!, $transport: String!, $url: String!, $clientId: String) {
    connectMcp(name: $name, transport: $transport, url: $url, clientId: $clientId) {
      name
      transport
      status
      toolCount
      success
      error
      requiresAuth
      url
    }
  }
`;

export const DISCONNECT_MCP_MUTATION = /* GraphQL */ `
  mutation DisconnectMcp($name: String!) {
    disconnectMcp(name: $name)
  }
`;

export const INITIATE_OAUTH_MUTATION = /* GraphQL */ `
  mutation InitiateOAuth($name: String!, $url: String!) {
    initiateOAuth(name: $name, url: $url) {
      success
      authUrl
      error
    }
  }
`;

// ─── Memory ────────────────────────────────────────────────────────────────────

export const ADD_MEMORY_NODE_MUTATION = /* GraphQL */ `
  mutation AddMemoryNode($label: String!, $type: String!, $value: String!) {
    addMemoryNode(label: $label, type: $type, value: $value) {
      id
      label
      type
      value
      createdAt
    }
  }
`;

export const UPDATE_MEMORY_NODE_MUTATION = /* GraphQL */ `
  mutation UpdateMemoryNode($id: String!, $label: String!, $type: String!, $value: String!, $properties: String) {
    updateMemoryNode(id: $id, label: $label, type: $type, value: $value, properties: $properties) {
      id
      label
      type
      value
      createdAt
    }
  }
`;

export const DELETE_MEMORY_NODE_MUTATION = /* GraphQL */ `
  mutation DeleteMemoryNode($id: String!) {
    deleteMemoryNode(id: $id)
  }
`;

// ─── Skills ────────────────────────────────────────────────────────────────────

export const ENABLE_SKILL_MUTATION = /* GraphQL */ `
  mutation EnableSkill($name: String!) {
    enableSkill(name: $name)
  }
`;

export const DISABLE_SKILL_MUTATION = /* GraphQL */ `
  mutation DisableSkill($name: String!) {
    disableSkill(name: $name)
  }
`;

export const DELETE_SKILL_MUTATION = /* GraphQL */ `
  mutation DeleteSkill($name: String!) {
    deleteSkill(name: $name)
  }
`;

export const IMPORT_SKILL_MUTATION = /* GraphQL */ `
  mutation ImportSkill($data: String!) {
    importSkill(data: $data) {
      success
      error
    }
  }
`;

// ─── Tool Permissions ──────────────────────────────────────────────────────────

export const SET_TOOL_PERMISSION_MUTATION = /* GraphQL */ `
  mutation SetToolPermission($userId: String!, $toolName: String!, $mode: String!) {
    setToolPermission(userId: $userId, toolName: $toolName, mode: $mode) {
      success
      error
    }
  }
`;

export const DELETE_TOOL_PERMISSION_MUTATION = /* GraphQL */ `
  mutation DeleteToolPermission($userId: String!, $toolName: String!) {
    deleteToolPermission(userId: $userId, toolName: $toolName) {
      success
      error
    }
  }
`;

export const SET_ALL_TOOL_PERMISSIONS_MUTATION = /* GraphQL */ `
  mutation SetAllToolPermissions($userId: String!, $mode: String!) {
    setAllToolPermissions(userId: $userId, mode: $mode) {
      success
      error
    }
  }
`;

export const DELETE_USER_MUTATION = /* GraphQL */ `
  mutation DeleteUser($conversationId: String!) {
    deleteUser(conversationId: $conversationId) {
      success
      error
    }
  }
`;

// ─── Configuration ─────────────────────────────────────────────────────────────

export const UPDATE_CONFIG_MUTATION = /* GraphQL */ `
  mutation UpdateConfig($input: UpdateConfigInput!) {
    updateConfig(input: $input) {
      agentName
      systemPrompt
      provider
      channels {
        channelId
        channelName
        enabled
      }
    }
  }
`;

// ─── System Files ─────────────────────────────────────────────────────────────

export const WRITE_SYSTEM_FILE_MUTATION = /* GraphQL */ `
  mutation WriteSystemFile($name: String!, $content: String!) {
    writeSystemFile(name: $name, content: $content) {
      success
      error
    }
  }
`;
