// Copyright (c) OpenLobster contributors. See LICENSE for details.

/*
 * GraphQL mutations for configuration updates and other backend operations
 */

export const UPDATE_CONFIG_MUTATION = `
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

export const ADD_MCP_SERVER_MUTATION = `
  mutation AddMCPServer($name: String!, $transport: String!, $command: String!) {
    addMCPServer(name: $name, transport: $transport, command: $command) {
      id
      name
      transport
      command
      status
    }
  }
`;

export const REMOVE_MCP_SERVER_MUTATION = `
  mutation RemoveMCPServer($id: String!) {
    removeMCPServer(id: $id) {
      success
      error
    }
  }
`;

export const ADD_TASK_MUTATION = `
  mutation AddTask($name: String!, $prompt: String!, $schedule: String!, $channel: String!, $isCyclic: Boolean!) {
    addTask(name: $name, prompt: $prompt, schedule: $schedule, channel: $channel, isCyclic: $isCyclic) {
      id
      name
      prompt
      schedule
      channel
      isCyclic
    }
  }
`;

export const REMOVE_TASK_MUTATION = `
  mutation RemoveTask($taskId: String!) {
    removeTask(taskId: $taskId) {
      success
      error
    }
  }
`;

// ─── Provider OAuth ─────────────────────────────────────────────────────────────

export const PROVIDER_OAUTH_PROVIDERS_QUERY = `
  query {
    providerOAuthProviders {
      id
      name
    }
  }
`;

export const PROVIDER_OAUTH_STATUS_QUERY = `
  query ProviderOAuthStatus($provider: String!) {
    providerOAuthStatus(provider: $provider) {
      provider
      status
      errorMessage
    }
  }
`;

export const PROVIDER_OAUTH_PROFILES_QUERY = `
  query ProviderOAuthProfiles($provider: String!) {
    providerOAuthProfiles(provider: $provider) {
      name
      authenticated
      accountID
    }
  }
`;

export const INITIATE_PROVIDER_OAUTH_MUTATION = `
  mutation InitiateProviderOAuth($provider: String!, $profileName: String) {
    initiateProviderOAuth(provider: $provider, profileName: $profileName) {
      authorizationURL
      instructions
    }
  }
`;

export const LOGOUT_PROVIDER_OAUTH_MUTATION = `
  mutation LogoutProviderOAuth($provider: String!) {
    logoutProviderOAuth(provider: $provider)
  }
`;

export const SET_ACTIVE_OAUTH_PROFILE_MUTATION = `
  mutation SetActiveOAuthProfile($provider: String!, $profileName: String!) {
    setActiveOAuthProfile(provider: $provider, profileName: $profileName)
  }
`;

export const DELETE_OAUTH_PROFILE_MUTATION = `
  mutation DeleteOAuthProfile($provider: String!, $profileName: String!) {
    deleteOAuthProfile(provider: $provider, profileName: $profileName)
  }
`;
