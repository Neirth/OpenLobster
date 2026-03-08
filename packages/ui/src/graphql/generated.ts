import type { GraphQLClient, RequestOptions } from 'graphql-request';
import gql from 'graphql-tag';
export type Maybe<T> = T | null | undefined;
export type InputMaybe<T> = T | null | undefined;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
type GraphQLClientRequestHeaders = RequestOptions['requestHeaders'];
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  JSON: { input: Record<string, unknown>; output: Record<string, unknown>; }
};

export type ActiveSession = {
  __typename?: 'ActiveSession';
  address?: Maybe<Scalars['String']['output']>;
  channel?: Maybe<Scalars['String']['output']>;
  id: Scalars['String']['output'];
  status?: Maybe<Scalars['String']['output']>;
  user?: Maybe<Scalars['String']['output']>;
};

export type AddRelationResult = {
  __typename?: 'AddRelationResult';
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type Agent = {
  __typename?: 'Agent';
  aiProvider?: Maybe<Scalars['String']['output']>;
  channels: Array<Channel>;
  id: Scalars['String']['output'];
  memoryBackend?: Maybe<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  provider?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  uptime: Scalars['Int']['output'];
  version: Scalars['String']['output'];
};

export type AgentConfig = {
  __typename?: 'AgentConfig';
  anthropicApiKey?: Maybe<Scalars['String']['output']>;
  apiKey?: Maybe<Scalars['String']['output']>;
  baseURL?: Maybe<Scalars['String']['output']>;
  dockerModelRunnerEndpoint?: Maybe<Scalars['String']['output']>;
  dockerModelRunnerModel?: Maybe<Scalars['String']['output']>;
  model?: Maybe<Scalars['String']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  ollamaApiKey?: Maybe<Scalars['String']['output']>;
  ollamaHost?: Maybe<Scalars['String']['output']>;
  provider?: Maybe<Scalars['String']['output']>;
  systemPrompt?: Maybe<Scalars['String']['output']>;
};

export type AppConfig = {
  __typename?: 'AppConfig';
  activeSessions: Array<ActiveSession>;
  agent?: Maybe<AgentConfig>;
  capabilities?: Maybe<CapabilitiesConfig>;
  channelSecrets?: Maybe<ChannelSecretsConfig>;
  channels: Array<ChannelConfig>;
  database?: Maybe<DatabaseConfig>;
  graphql?: Maybe<GraphQlConfig>;
  logging?: Maybe<LoggingConfig>;
  memory?: Maybe<MemoryConfig>;
  scheduler?: Maybe<SchedulerConfig>;
  secrets?: Maybe<SecretsConfig>;
  subagents?: Maybe<SubagentsConfig>;
  wizardCompleted?: Maybe<Scalars['Boolean']['output']>;
};

export type ApprovePairingResult = {
  __typename?: 'ApprovePairingResult';
  error?: Maybe<Scalars['String']['output']>;
  pairing?: Maybe<PairingInfo>;
  success: Scalars['Boolean']['output'];
};

export type CapabilitiesConfig = {
  __typename?: 'CapabilitiesConfig';
  browser?: Maybe<Scalars['Boolean']['output']>;
  filesystem?: Maybe<Scalars['Boolean']['output']>;
  mcp?: Maybe<Scalars['Boolean']['output']>;
  memory?: Maybe<Scalars['Boolean']['output']>;
  sessions?: Maybe<Scalars['Boolean']['output']>;
  subagents?: Maybe<Scalars['Boolean']['output']>;
  terminal?: Maybe<Scalars['Boolean']['output']>;
};

export type CapabilitiesInput = {
  browser?: InputMaybe<Scalars['Boolean']['input']>;
  filesystem?: InputMaybe<Scalars['Boolean']['input']>;
  mcp?: InputMaybe<Scalars['Boolean']['input']>;
  memory?: InputMaybe<Scalars['Boolean']['input']>;
  sessions?: InputMaybe<Scalars['Boolean']['input']>;
  subagents?: InputMaybe<Scalars['Boolean']['input']>;
  terminal?: InputMaybe<Scalars['Boolean']['input']>;
};

export type Channel = {
  __typename?: 'Channel';
  capabilities?: Maybe<ChannelCapabilities>;
  enabled: Scalars['Boolean']['output'];
  id: Scalars['String']['output'];
  messagesReceived?: Maybe<Scalars['Int']['output']>;
  messagesSent?: Maybe<Scalars['Int']['output']>;
  name: Scalars['String']['output'];
  status: Scalars['String']['output'];
  type: Scalars['String']['output'];
};

export type ChannelCapabilities = {
  __typename?: 'ChannelCapabilities';
  hasCallStream: Scalars['Boolean']['output'];
  hasMediaSupport: Scalars['Boolean']['output'];
  hasTextStream: Scalars['Boolean']['output'];
  hasVoiceMessage: Scalars['Boolean']['output'];
};

export type ChannelConfig = {
  __typename?: 'ChannelConfig';
  channelId: Scalars['String']['output'];
  channelName?: Maybe<Scalars['String']['output']>;
  enabled: Scalars['Boolean']['output'];
};

export type ChannelSecretsConfig = {
  __typename?: 'ChannelSecretsConfig';
  discordEnabled?: Maybe<Scalars['Boolean']['output']>;
  discordToken?: Maybe<Scalars['String']['output']>;
  slackAppToken?: Maybe<Scalars['String']['output']>;
  slackBotToken?: Maybe<Scalars['String']['output']>;
  slackEnabled?: Maybe<Scalars['Boolean']['output']>;
  telegramEnabled?: Maybe<Scalars['Boolean']['output']>;
  telegramToken?: Maybe<Scalars['String']['output']>;
  twilioAccountSid?: Maybe<Scalars['String']['output']>;
  twilioAuthToken?: Maybe<Scalars['String']['output']>;
  twilioEnabled?: Maybe<Scalars['Boolean']['output']>;
  twilioFromNumber?: Maybe<Scalars['String']['output']>;
  whatsAppApiToken?: Maybe<Scalars['String']['output']>;
  whatsAppEnabled?: Maybe<Scalars['Boolean']['output']>;
  whatsAppPhoneId?: Maybe<Scalars['String']['output']>;
};

export type Conversation = {
  __typename?: 'Conversation';
  channelId: Scalars['String']['output'];
  channelName?: Maybe<Scalars['String']['output']>;
  id: Scalars['String']['output'];
  isGroup: Scalars['Boolean']['output'];
  lastMessageAt?: Maybe<Scalars['String']['output']>;
  participantId?: Maybe<Scalars['String']['output']>;
  participantName?: Maybe<Scalars['String']['output']>;
  unreadCount?: Maybe<Scalars['Int']['output']>;
};

export type CypherResult = {
  __typename?: 'CypherResult';
  data?: Maybe<Scalars['JSON']['output']>;
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type DatabaseConfig = {
  __typename?: 'DatabaseConfig';
  driver?: Maybe<Scalars['String']['output']>;
  dsn?: Maybe<Scalars['String']['output']>;
  maxIdleConns?: Maybe<Scalars['Int']['output']>;
  maxOpenConns?: Maybe<Scalars['Int']['output']>;
};

export type DenyPairingResult = {
  __typename?: 'DenyPairingResult';
  code?: Maybe<Scalars['String']['output']>;
  error?: Maybe<Scalars['String']['output']>;
  reason?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type EventPayload = {
  __typename?: 'EventPayload';
  data?: Maybe<Scalars['JSON']['output']>;
  timestamp: Scalars['String']['output'];
  type: Scalars['String']['output'];
};

export type FileSecretsConfig = {
  __typename?: 'FileSecretsConfig';
  path?: Maybe<Scalars['String']['output']>;
};

export type GraphEdge = {
  __typename?: 'GraphEdge';
  label?: Maybe<Scalars['String']['output']>;
  source: Scalars['String']['output'];
  target: Scalars['String']['output'];
};

export type GraphNode = {
  __typename?: 'GraphNode';
  id: Scalars['String']['output'];
  label?: Maybe<Scalars['String']['output']>;
  properties?: Maybe<Scalars['JSON']['output']>;
  type?: Maybe<Scalars['String']['output']>;
  value?: Maybe<Scalars['String']['output']>;
};

export type GraphQlConfig = {
  __typename?: 'GraphQLConfig';
  baseUrl?: Maybe<Scalars['String']['output']>;
  enabled?: Maybe<Scalars['Boolean']['output']>;
  host?: Maybe<Scalars['String']['output']>;
  port?: Maybe<Scalars['Int']['output']>;
};

export type Heartbeat = {
  __typename?: 'Heartbeat';
  lastCheck: Scalars['Int']['output'];
  status: Scalars['String']['output'];
};

export type ImportSkillResult = {
  __typename?: 'ImportSkillResult';
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type KillSubAgentResult = {
  __typename?: 'KillSubAgentResult';
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type LoggingConfig = {
  __typename?: 'LoggingConfig';
  level?: Maybe<Scalars['String']['output']>;
  path?: Maybe<Scalars['String']['output']>;
};

export type Mcp = {
  __typename?: 'MCP';
  name: Scalars['String']['output'];
  status?: Maybe<Scalars['String']['output']>;
  tools: Array<Tool>;
  type?: Maybe<Scalars['String']['output']>;
  url?: Maybe<Scalars['String']['output']>;
};

export type McpConnectResult = {
  __typename?: 'MCPConnectResult';
  error?: Maybe<Scalars['String']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  requiresAuth?: Maybe<Scalars['Boolean']['output']>;
  status?: Maybe<Scalars['String']['output']>;
  success?: Maybe<Scalars['Boolean']['output']>;
  toolCount?: Maybe<Scalars['Int']['output']>;
  transport?: Maybe<Scalars['String']['output']>;
  url?: Maybe<Scalars['String']['output']>;
};

export type McpoAuthStatus = {
  __typename?: 'MCPOAuthStatus';
  error?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
};

export type McpServer = {
  __typename?: 'MCPServer';
  name: Scalars['String']['output'];
  requiresAuth?: Maybe<Scalars['Boolean']['output']>;
  status: Scalars['String']['output'];
  toolCount: Scalars['Int']['output'];
  transport?: Maybe<Scalars['String']['output']>;
  url?: Maybe<Scalars['String']['output']>;
};

export type McpTool = {
  __typename?: 'MCPTool';
  description?: Maybe<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  serverName?: Maybe<Scalars['String']['output']>;
};

export type McpUser = {
  __typename?: 'MCPUser';
  channelId: Scalars['String']['output'];
  displayName?: Maybe<Scalars['String']['output']>;
  isAgent: Scalars['Boolean']['output'];
};

export type MemoryConfig = {
  __typename?: 'MemoryConfig';
  backend?: Maybe<Scalars['String']['output']>;
  filePath?: Maybe<Scalars['String']['output']>;
  neo4j?: Maybe<Neo4jConfig>;
};

export type MemoryEdge = {
  __typename?: 'MemoryEdge';
  id: Scalars['String']['output'];
  relation?: Maybe<Scalars['String']['output']>;
  sourceId: Scalars['String']['output'];
  targetId: Scalars['String']['output'];
};

export type MemoryGraph = {
  __typename?: 'MemoryGraph';
  edges: Array<MemoryEdge>;
  nodes: Array<MemoryNode>;
};

export type MemoryNode = {
  __typename?: 'MemoryNode';
  createdAt?: Maybe<Scalars['String']['output']>;
  id: Scalars['String']['output'];
  label?: Maybe<Scalars['String']['output']>;
  properties?: Maybe<Scalars['JSON']['output']>;
  type?: Maybe<Scalars['String']['output']>;
  value?: Maybe<Scalars['String']['output']>;
};

export type Message = {
  __typename?: 'Message';
  content: Scalars['String']['output'];
  conversationId: Scalars['String']['output'];
  createdAt: Scalars['String']['output'];
  id: Scalars['String']['output'];
  role: Scalars['String']['output'];
};

export type MessageSentResult = {
  __typename?: 'MessageSentResult';
  content?: Maybe<Scalars['String']['output']>;
  conversationId?: Maybe<Scalars['String']['output']>;
  createdAt?: Maybe<Scalars['String']['output']>;
  error?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['String']['output']>;
  role?: Maybe<Scalars['String']['output']>;
  success?: Maybe<Scalars['Boolean']['output']>;
};

export type Metrics = {
  __typename?: 'Metrics';
  activeSessions: Scalars['Int']['output'];
  errorsTotal: Scalars['Int']['output'];
  mcpTools: Scalars['Int']['output'];
  memoryEdges: Scalars['Int']['output'];
  memoryNodes: Scalars['Int']['output'];
  messagesReceived: Scalars['Int']['output'];
  messagesSent: Scalars['Int']['output'];
  tasksDone: Scalars['Int']['output'];
  tasksPending: Scalars['Int']['output'];
  tasksRunning: Scalars['Int']['output'];
  uptime: Scalars['Int']['output'];
};

export type Mutation = {
  __typename?: 'Mutation';
  addMemory: MutationResult;
  addMemoryNode: MemoryNode;
  addRelation: AddRelationResult;
  addTask: Task;
  approvePairing: ApprovePairingResult;
  completeTask: Scalars['Boolean']['output'];
  connectMcp: McpConnectResult;
  deleteMemoryNode: Scalars['Boolean']['output'];
  deleteSkill: Scalars['Boolean']['output'];
  deleteToolPermission: MutationResult;
  deleteUser: MutationResult;
  denyPairing: DenyPairingResult;
  disableSkill: Scalars['Boolean']['output'];
  disconnectMcp: Scalars['Boolean']['output'];
  enableSkill: Scalars['Boolean']['output'];
  executeCypher: CypherResult;
  importSkill: ImportSkillResult;
  initiateOAuth: OAuthInitiateResult;
  killSubAgent: KillSubAgentResult;
  removeTask: Scalars['Boolean']['output'];
  sendMessage: MessageSentResult;
  setAllToolPermissions: MutationResult;
  setToolPermission: MutationResult;
  spawnSubAgent: SpawnSubAgentResult;
  toggleTask: ToggleTaskResult;
  updateConfig: UpdateConfigResult;
  updateMemoryNode: MemoryNode;
  updateTask?: Maybe<Task>;
  writeSystemFile: MutationResult;
};


export type MutationAddMemoryArgs = {
  content: Scalars['String']['input'];
};


export type MutationAddMemoryNodeArgs = {
  label: Scalars['String']['input'];
  type: Scalars['String']['input'];
  value: Scalars['String']['input'];
};


export type MutationAddRelationArgs = {
  from: Scalars['String']['input'];
  relationType: Scalars['String']['input'];
  to: Scalars['String']['input'];
};


export type MutationAddTaskArgs = {
  prompt: Scalars['String']['input'];
  schedule?: InputMaybe<Scalars['String']['input']>;
};


export type MutationApprovePairingArgs = {
  code: Scalars['String']['input'];
  displayName?: InputMaybe<Scalars['String']['input']>;
  userID?: InputMaybe<Scalars['String']['input']>;
};


export type MutationCompleteTaskArgs = {
  taskId: Scalars['String']['input'];
};


export type MutationConnectMcpArgs = {
  name: Scalars['String']['input'];
  transport?: InputMaybe<Scalars['String']['input']>;
  url?: InputMaybe<Scalars['String']['input']>;
};


export type MutationDeleteMemoryNodeArgs = {
  id: Scalars['String']['input'];
};


export type MutationDeleteSkillArgs = {
  name: Scalars['String']['input'];
};


export type MutationDeleteToolPermissionArgs = {
  toolName: Scalars['String']['input'];
  userId: Scalars['String']['input'];
};


export type MutationDeleteUserArgs = {
  conversationId: Scalars['String']['input'];
};


export type MutationDenyPairingArgs = {
  code: Scalars['String']['input'];
  reason?: InputMaybe<Scalars['String']['input']>;
};


export type MutationDisableSkillArgs = {
  name: Scalars['String']['input'];
};


export type MutationDisconnectMcpArgs = {
  name: Scalars['String']['input'];
};


export type MutationEnableSkillArgs = {
  name: Scalars['String']['input'];
};


export type MutationExecuteCypherArgs = {
  cypher: Scalars['String']['input'];
};


export type MutationImportSkillArgs = {
  data: Scalars['String']['input'];
};


export type MutationInitiateOAuthArgs = {
  name: Scalars['String']['input'];
  url: Scalars['String']['input'];
};


export type MutationKillSubAgentArgs = {
  id: Scalars['String']['input'];
};


export type MutationRemoveTaskArgs = {
  taskId: Scalars['String']['input'];
};


export type MutationSendMessageArgs = {
  channelId?: InputMaybe<Scalars['String']['input']>;
  content: Scalars['String']['input'];
  conversationId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationSetAllToolPermissionsArgs = {
  mode: Scalars['String']['input'];
  userId: Scalars['String']['input'];
};


export type MutationSetToolPermissionArgs = {
  mode: Scalars['String']['input'];
  toolName: Scalars['String']['input'];
  userId: Scalars['String']['input'];
};


export type MutationSpawnSubAgentArgs = {
  model: Scalars['String']['input'];
  name: Scalars['String']['input'];
  task?: InputMaybe<Scalars['String']['input']>;
};


export type MutationToggleTaskArgs = {
  enabled: Scalars['Boolean']['input'];
  id: Scalars['String']['input'];
};


export type MutationUpdateConfigArgs = {
  input: UpdateConfigInput;
};


export type MutationUpdateMemoryNodeArgs = {
  id: Scalars['String']['input'];
  label?: InputMaybe<Scalars['String']['input']>;
  properties?: InputMaybe<Scalars['String']['input']>;
  type?: InputMaybe<Scalars['String']['input']>;
  value?: InputMaybe<Scalars['String']['input']>;
};


export type MutationUpdateTaskArgs = {
  id: Scalars['String']['input'];
  prompt: Scalars['String']['input'];
  schedule?: InputMaybe<Scalars['String']['input']>;
};


export type MutationWriteSystemFileArgs = {
  content: Scalars['String']['input'];
  name: Scalars['String']['input'];
};

export type MutationResult = {
  __typename?: 'MutationResult';
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type Neo4jConfig = {
  __typename?: 'Neo4jConfig';
  password?: Maybe<Scalars['String']['output']>;
  uri?: Maybe<Scalars['String']['output']>;
  user?: Maybe<Scalars['String']['output']>;
};

export type OAuthInitiateResult = {
  __typename?: 'OAuthInitiateResult';
  authUrl?: Maybe<Scalars['String']['output']>;
  error?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type OpenbaoSecretsConfig = {
  __typename?: 'OpenbaoSecretsConfig';
  token?: Maybe<Scalars['String']['output']>;
  url?: Maybe<Scalars['String']['output']>;
};

export type PairingInfo = {
  __typename?: 'PairingInfo';
  code: Scalars['String']['output'];
  status: Scalars['String']['output'];
};

export type PendingPairing = {
  __typename?: 'PendingPairing';
  channelID?: Maybe<Scalars['String']['output']>;
  channelType?: Maybe<Scalars['String']['output']>;
  code: Scalars['String']['output'];
  createdAt?: Maybe<Scalars['String']['output']>;
  expiresAt?: Maybe<Scalars['String']['output']>;
  platformUserName?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
};

export type Query = {
  __typename?: 'Query';
  agent?: Maybe<Agent>;
  channels: Array<Channel>;
  config?: Maybe<AppConfig>;
  conversations: Array<Conversation>;
  heartbeat?: Maybe<Heartbeat>;
  mcpOAuthStatus?: Maybe<McpoAuthStatus>;
  mcpServers: Array<McpServer>;
  mcpTools: Array<McpTool>;
  mcpUsers: Array<McpUser>;
  mcps: Array<Mcp>;
  memory?: Maybe<MemoryGraph>;
  messages: Array<Message>;
  metrics?: Maybe<Metrics>;
  pendingPairings: Array<PendingPairing>;
  searchMemory?: Maybe<SearchMemoryResult>;
  skills: Array<Skill>;
  status?: Maybe<Status>;
  subAgents: Array<SubAgent>;
  systemFiles: Array<SystemFile>;
  tasks: Array<Task>;
  toolPermissions: Array<ToolPermission>;
  tools: Array<Tool>;
  userGraph?: Maybe<UserGraphResult>;
  users: Array<User>;
};


export type QueryMcpOAuthStatusArgs = {
  name: Scalars['String']['input'];
};


export type QueryMessagesArgs = {
  conversationId: Scalars['String']['input'];
};


export type QuerySearchMemoryArgs = {
  query: Scalars['String']['input'];
};


export type QueryToolPermissionsArgs = {
  userId: Scalars['String']['input'];
};


export type QueryUserGraphArgs = {
  userId?: InputMaybe<Scalars['String']['input']>;
};

export type SchedulerConfig = {
  __typename?: 'SchedulerConfig';
  enabled?: Maybe<Scalars['Boolean']['output']>;
  memoryEnabled?: Maybe<Scalars['Boolean']['output']>;
  memoryInterval?: Maybe<Scalars['String']['output']>;
};

export type SearchMemoryResult = {
  __typename?: 'SearchMemoryResult';
  error?: Maybe<Scalars['String']['output']>;
  result?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type SecretsConfig = {
  __typename?: 'SecretsConfig';
  backend?: Maybe<Scalars['String']['output']>;
  file?: Maybe<FileSecretsConfig>;
  openbao?: Maybe<OpenbaoSecretsConfig>;
};

export type Skill = {
  __typename?: 'Skill';
  description?: Maybe<Scalars['String']['output']>;
  enabled: Scalars['Boolean']['output'];
  name: Scalars['String']['output'];
  path?: Maybe<Scalars['String']['output']>;
};

export type SpawnSubAgentResult = {
  __typename?: 'SpawnSubAgentResult';
  error?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type Status = {
  __typename?: 'Status';
  agent?: Maybe<Agent>;
  channels: Array<Channel>;
  health?: Maybe<Heartbeat>;
  mcps: Array<Mcp>;
  subAgents: Array<SubAgent>;
  tasks: Array<Task>;
  tools: Array<Tool>;
};

export type SubAgent = {
  __typename?: 'SubAgent';
  id: Scalars['String']['output'];
  name: Scalars['String']['output'];
  status: Scalars['String']['output'];
  task?: Maybe<Scalars['String']['output']>;
};

export type SubagentsConfig = {
  __typename?: 'SubagentsConfig';
  defaultTimeout?: Maybe<Scalars['String']['output']>;
  maxConcurrent?: Maybe<Scalars['Int']['output']>;
};

export type Subscription = {
  __typename?: 'Subscription';
  events?: Maybe<EventPayload>;
  onCompactionCompleted?: Maybe<EventPayload>;
  onCompactionTriggered?: Maybe<EventPayload>;
  onCronJobExecuted?: Maybe<EventPayload>;
  onMCPServerConnected?: Maybe<EventPayload>;
  onMCPServerDisconnected?: Maybe<EventPayload>;
  onMemoryUpdated?: Maybe<EventPayload>;
  onMessageProcessed?: Maybe<EventPayload>;
  onMessageReceived?: Maybe<EventPayload>;
  onMessageSent?: Maybe<EventPayload>;
  onPairingApproved?: Maybe<EventPayload>;
  onPairingDenied?: Maybe<EventPayload>;
  onPairingRequested?: Maybe<EventPayload>;
  onSessionEnded?: Maybe<EventPayload>;
  onSessionStarted?: Maybe<EventPayload>;
  onTaskAdded?: Maybe<EventPayload>;
  onTaskCompleted?: Maybe<EventPayload>;
  onUserPaired?: Maybe<EventPayload>;
  onUserUnpaired?: Maybe<EventPayload>;
};


export type SubscriptionEventsArgs = {
  eventType?: InputMaybe<Scalars['String']['input']>;
};

export type SystemFile = {
  __typename?: 'SystemFile';
  content?: Maybe<Scalars['String']['output']>;
  lastModified?: Maybe<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  path: Scalars['String']['output'];
};

export type Task = {
  __typename?: 'Task';
  createdAt?: Maybe<Scalars['String']['output']>;
  enabled: Scalars['Boolean']['output'];
  id: Scalars['String']['output'];
  isCyclic?: Maybe<Scalars['Boolean']['output']>;
  lastRunAt?: Maybe<Scalars['String']['output']>;
  nextRunAt?: Maybe<Scalars['String']['output']>;
  prompt: Scalars['String']['output'];
  schedule?: Maybe<Scalars['String']['output']>;
  status: Scalars['String']['output'];
  taskType?: Maybe<Scalars['String']['output']>;
};

export type ToggleTaskResult = {
  __typename?: 'ToggleTaskResult';
  enabled?: Maybe<Scalars['Boolean']['output']>;
  error?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['String']['output']>;
  success: Scalars['Boolean']['output'];
};

export type Tool = {
  __typename?: 'Tool';
  description?: Maybe<Scalars['String']['output']>;
  name: Scalars['String']['output'];
  source?: Maybe<Scalars['String']['output']>;
};

export type ToolPermission = {
  __typename?: 'ToolPermission';
  mode: Scalars['String']['output'];
  toolName: Scalars['String']['output'];
};

export type UpdateConfigInput = {
  agentName?: InputMaybe<Scalars['String']['input']>;
  anthropicApiKey?: InputMaybe<Scalars['String']['input']>;
  apiKey?: InputMaybe<Scalars['String']['input']>;
  baseURL?: InputMaybe<Scalars['String']['input']>;
  capabilities?: InputMaybe<CapabilitiesInput>;
  channelDiscordEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  channelDiscordToken?: InputMaybe<Scalars['String']['input']>;
  channelSlackAppToken?: InputMaybe<Scalars['String']['input']>;
  channelSlackBotToken?: InputMaybe<Scalars['String']['input']>;
  channelSlackEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  channelTelegramEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  channelTelegramToken?: InputMaybe<Scalars['String']['input']>;
  channelTwilioAccountSid?: InputMaybe<Scalars['String']['input']>;
  channelTwilioAuthToken?: InputMaybe<Scalars['String']['input']>;
  channelTwilioEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  channelTwilioFromNumber?: InputMaybe<Scalars['String']['input']>;
  channelWhatsAppApiToken?: InputMaybe<Scalars['String']['input']>;
  channelWhatsAppEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  channelWhatsAppPhoneId?: InputMaybe<Scalars['String']['input']>;
  databaseDSN?: InputMaybe<Scalars['String']['input']>;
  databaseDriver?: InputMaybe<Scalars['String']['input']>;
  databaseMaxIdleConns?: InputMaybe<Scalars['Int']['input']>;
  databaseMaxOpenConns?: InputMaybe<Scalars['Int']['input']>;
  dockerModelRunnerEndpoint?: InputMaybe<Scalars['String']['input']>;
  dockerModelRunnerModel?: InputMaybe<Scalars['String']['input']>;
  graphqlBaseUrl?: InputMaybe<Scalars['String']['input']>;
  graphqlEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  graphqlHost?: InputMaybe<Scalars['String']['input']>;
  graphqlPort?: InputMaybe<Scalars['Int']['input']>;
  loggingLevel?: InputMaybe<Scalars['String']['input']>;
  loggingPath?: InputMaybe<Scalars['String']['input']>;
  memoryBackend?: InputMaybe<Scalars['String']['input']>;
  memoryFilePath?: InputMaybe<Scalars['String']['input']>;
  memoryNeo4jPassword?: InputMaybe<Scalars['String']['input']>;
  memoryNeo4jURI?: InputMaybe<Scalars['String']['input']>;
  memoryNeo4jUser?: InputMaybe<Scalars['String']['input']>;
  model?: InputMaybe<Scalars['String']['input']>;
  ollamaApiKey?: InputMaybe<Scalars['String']['input']>;
  ollamaHost?: InputMaybe<Scalars['String']['input']>;
  provider?: InputMaybe<Scalars['String']['input']>;
  schedulerEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  schedulerMemoryEnabled?: InputMaybe<Scalars['Boolean']['input']>;
  schedulerMemoryInterval?: InputMaybe<Scalars['String']['input']>;
  secretsBackend?: InputMaybe<Scalars['String']['input']>;
  secretsFilePath?: InputMaybe<Scalars['String']['input']>;
  secretsOpenbaoToken?: InputMaybe<Scalars['String']['input']>;
  secretsOpenbaoURL?: InputMaybe<Scalars['String']['input']>;
  subagentsDefaultTimeout?: InputMaybe<Scalars['String']['input']>;
  subagentsMaxConcurrent?: InputMaybe<Scalars['Int']['input']>;
  systemPrompt?: InputMaybe<Scalars['String']['input']>;
  wizardCompleted?: InputMaybe<Scalars['Boolean']['input']>;
};

export type UpdateConfigResult = {
  __typename?: 'UpdateConfigResult';
  agentName?: Maybe<Scalars['String']['output']>;
  channels?: Maybe<Array<ChannelConfig>>;
  provider?: Maybe<Scalars['String']['output']>;
  systemPrompt?: Maybe<Scalars['String']['output']>;
};

export type User = {
  __typename?: 'User';
  createdAt?: Maybe<Scalars['Int']['output']>;
  id: Scalars['String']['output'];
  primaryID?: Maybe<Scalars['String']['output']>;
};

export type UserGraphResult = {
  __typename?: 'UserGraphResult';
  edges?: Maybe<Array<GraphEdge>>;
  error?: Maybe<Scalars['String']['output']>;
  nodes?: Maybe<Array<GraphNode>>;
  success: Scalars['Boolean']['output'];
};

export type SendMessageMutationVariables = Exact<{
  conversationId: Scalars['String']['input'];
  content: Scalars['String']['input'];
}>;


export type SendMessageMutation = { __typename?: 'Mutation', sendMessage: { __typename?: 'MessageSentResult', id?: string | null | undefined, conversationId?: string | null | undefined, role?: string | null | undefined, content?: string | null | undefined, createdAt?: string | null | undefined } };

export type AddTaskMutationVariables = Exact<{
  prompt: Scalars['String']['input'];
  schedule?: InputMaybe<Scalars['String']['input']>;
}>;


export type AddTaskMutation = { __typename?: 'Mutation', addTask: { __typename?: 'Task', id: string, prompt: string, status: string, schedule?: string | null | undefined, taskType?: string | null | undefined, isCyclic?: boolean | null | undefined, createdAt?: string | null | undefined, lastRunAt?: string | null | undefined, nextRunAt?: string | null | undefined } };

export type CompleteTaskMutationVariables = Exact<{
  taskId: Scalars['String']['input'];
}>;


export type CompleteTaskMutation = { __typename?: 'Mutation', completeTask: boolean };

export type RemoveTaskMutationVariables = Exact<{
  taskId: Scalars['String']['input'];
}>;


export type RemoveTaskMutation = { __typename?: 'Mutation', removeTask: boolean };

export type ToggleTaskMutationVariables = Exact<{
  id: Scalars['String']['input'];
  enabled: Scalars['Boolean']['input'];
}>;


export type ToggleTaskMutation = { __typename?: 'Mutation', toggleTask: { __typename?: 'ToggleTaskResult', success: boolean, id?: string | null | undefined, enabled?: boolean | null | undefined } };

export type UpdateTaskMutationVariables = Exact<{
  id: Scalars['String']['input'];
  prompt: Scalars['String']['input'];
  schedule?: InputMaybe<Scalars['String']['input']>;
}>;


export type UpdateTaskMutation = { __typename?: 'Mutation', updateTask?: { __typename?: 'Task', id: string, prompt: string, status: string, schedule?: string | null | undefined, taskType?: string | null | undefined, isCyclic?: boolean | null | undefined, createdAt?: string | null | undefined, lastRunAt?: string | null | undefined, nextRunAt?: string | null | undefined } | null | undefined };

export type ConnectMcpMutationVariables = Exact<{
  name: Scalars['String']['input'];
  transport: Scalars['String']['input'];
  url?: InputMaybe<Scalars['String']['input']>;
}>;


export type ConnectMcpMutation = { __typename?: 'Mutation', connectMcp: { __typename?: 'MCPConnectResult', name?: string | null | undefined, transport?: string | null | undefined, status?: string | null | undefined, toolCount?: number | null | undefined, success?: boolean | null | undefined, error?: string | null | undefined, requiresAuth?: boolean | null | undefined, url?: string | null | undefined } };

export type DisconnectMcpMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type DisconnectMcpMutation = { __typename?: 'Mutation', disconnectMcp: boolean };

export type InitiateOAuthMutationVariables = Exact<{
  name: Scalars['String']['input'];
  url: Scalars['String']['input'];
}>;


export type InitiateOAuthMutation = { __typename?: 'Mutation', initiateOAuth: { __typename?: 'OAuthInitiateResult', success: boolean, authUrl?: string | null | undefined, error?: string | null | undefined } };

export type AddMemoryNodeMutationVariables = Exact<{
  label: Scalars['String']['input'];
  type: Scalars['String']['input'];
  value: Scalars['String']['input'];
}>;


export type AddMemoryNodeMutation = { __typename?: 'Mutation', addMemoryNode: { __typename?: 'MemoryNode', id: string, label?: string | null | undefined, type?: string | null | undefined, value?: string | null | undefined, createdAt?: string | null | undefined } };

export type UpdateMemoryNodeMutationVariables = Exact<{
  id: Scalars['String']['input'];
  label: Scalars['String']['input'];
  type: Scalars['String']['input'];
  value: Scalars['String']['input'];
  properties?: InputMaybe<Scalars['String']['input']>;
}>;


export type UpdateMemoryNodeMutation = { __typename?: 'Mutation', updateMemoryNode: { __typename?: 'MemoryNode', id: string, label?: string | null | undefined, type?: string | null | undefined, value?: string | null | undefined, createdAt?: string | null | undefined } };

export type DeleteMemoryNodeMutationVariables = Exact<{
  id: Scalars['String']['input'];
}>;


export type DeleteMemoryNodeMutation = { __typename?: 'Mutation', deleteMemoryNode: boolean };

export type EnableSkillMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type EnableSkillMutation = { __typename?: 'Mutation', enableSkill: boolean };

export type DisableSkillMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type DisableSkillMutation = { __typename?: 'Mutation', disableSkill: boolean };

export type DeleteSkillMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type DeleteSkillMutation = { __typename?: 'Mutation', deleteSkill: boolean };

export type ImportSkillMutationVariables = Exact<{
  data: Scalars['String']['input'];
}>;


export type ImportSkillMutation = { __typename?: 'Mutation', importSkill: { __typename?: 'ImportSkillResult', success: boolean, error?: string | null | undefined } };

export type SetToolPermissionMutationVariables = Exact<{
  userId: Scalars['String']['input'];
  toolName: Scalars['String']['input'];
  mode: Scalars['String']['input'];
}>;


export type SetToolPermissionMutation = { __typename?: 'Mutation', setToolPermission: { __typename?: 'MutationResult', success: boolean, error?: string | null | undefined } };

export type DeleteToolPermissionMutationVariables = Exact<{
  userId: Scalars['String']['input'];
  toolName: Scalars['String']['input'];
}>;


export type DeleteToolPermissionMutation = { __typename?: 'Mutation', deleteToolPermission: { __typename?: 'MutationResult', success: boolean, error?: string | null | undefined } };

export type SetAllToolPermissionsMutationVariables = Exact<{
  userId: Scalars['String']['input'];
  mode: Scalars['String']['input'];
}>;


export type SetAllToolPermissionsMutation = { __typename?: 'Mutation', setAllToolPermissions: { __typename?: 'MutationResult', success: boolean, error?: string | null | undefined } };

export type DeleteUserMutationVariables = Exact<{
  conversationId: Scalars['String']['input'];
}>;


export type DeleteUserMutation = { __typename?: 'Mutation', deleteUser: { __typename?: 'MutationResult', success: boolean, error?: string | null | undefined } };

export type UpdateConfigMutationVariables = Exact<{
  input: UpdateConfigInput;
}>;


export type UpdateConfigMutation = { __typename?: 'Mutation', updateConfig: { __typename?: 'UpdateConfigResult', agentName?: string | null | undefined, systemPrompt?: string | null | undefined, provider?: string | null | undefined, channels?: Array<{ __typename?: 'ChannelConfig', channelId: string, channelName?: string | null | undefined, enabled: boolean }> | null | undefined } };

export type WriteSystemFileMutationVariables = Exact<{
  name: Scalars['String']['input'];
  content: Scalars['String']['input'];
}>;


export type WriteSystemFileMutation = { __typename?: 'Mutation', writeSystemFile: { __typename?: 'MutationResult', success: boolean, error?: string | null | undefined } };

export type GetAgentQueryVariables = Exact<{ [key: string]: never; }>;


export type GetAgentQuery = { __typename?: 'Query', agent?: { __typename?: 'Agent', id: string, name: string, version: string, status: string, uptime: number, provider?: string | null | undefined } | null | undefined };

export type GetMetricsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMetricsQuery = { __typename?: 'Query', metrics?: { __typename?: 'Metrics', uptime: number, messagesReceived: number, messagesSent: number, activeSessions: number, memoryNodes: number, memoryEdges: number, mcpTools: number, tasksPending: number, tasksRunning: number, tasksDone: number, errorsTotal: number } | null | undefined };

export type GetChannelsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetChannelsQuery = { __typename?: 'Query', channels: Array<{ __typename?: 'Channel', id: string, name: string, type: string, status: string, messagesReceived?: number | null | undefined, messagesSent?: number | null | undefined }> };

export type GetConversationsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetConversationsQuery = { __typename?: 'Query', conversations: Array<{ __typename?: 'Conversation', id: string, channelId: string, channelName?: string | null | undefined, isGroup: boolean, participantId?: string | null | undefined, participantName?: string | null | undefined, lastMessageAt?: string | null | undefined, unreadCount?: number | null | undefined }> };

export type GetMessagesQueryVariables = Exact<{
  conversationId: Scalars['String']['input'];
}>;


export type GetMessagesQuery = { __typename?: 'Query', messages: Array<{ __typename?: 'Message', id: string, conversationId: string, role: string, content: string, createdAt: string }> };

export type GetTasksQueryVariables = Exact<{ [key: string]: never; }>;


export type GetTasksQuery = { __typename?: 'Query', tasks: Array<{ __typename?: 'Task', id: string, prompt: string, status: string, schedule?: string | null | undefined, taskType?: string | null | undefined, isCyclic?: boolean | null | undefined, enabled: boolean, createdAt?: string | null | undefined, lastRunAt?: string | null | undefined, nextRunAt?: string | null | undefined }> };

export type GetMcpServersQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMcpServersQuery = { __typename?: 'Query', mcpServers: Array<{ __typename?: 'MCPServer', name: string, transport?: string | null | undefined, status: string, toolCount: number, url?: string | null | undefined }> };

export type GetMcpToolsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMcpToolsQuery = { __typename?: 'Query', mcpTools: Array<{ __typename?: 'MCPTool', name: string, serverName?: string | null | undefined, description?: string | null | undefined }> };

export type GetMcpUsersQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMcpUsersQuery = { __typename?: 'Query', mcpUsers: Array<{ __typename?: 'MCPUser', channelId: string, displayName?: string | null | undefined, isAgent: boolean }> };

export type GetToolPermissionsQueryVariables = Exact<{
  userId: Scalars['String']['input'];
}>;


export type GetToolPermissionsQuery = { __typename?: 'Query', toolPermissions: Array<{ __typename?: 'ToolPermission', toolName: string, mode: string }> };

export type GetMemoryQueryVariables = Exact<{ [key: string]: never; }>;


export type GetMemoryQuery = { __typename?: 'Query', memory?: { __typename?: 'MemoryGraph', nodes: Array<{ __typename?: 'MemoryNode', id: string, label?: string | null | undefined, type?: string | null | undefined, value?: string | null | undefined, createdAt?: string | null | undefined, properties?: Record<string, unknown> | null | undefined }>, edges: Array<{ __typename?: 'MemoryEdge', id: string, sourceId: string, targetId: string, relation?: string | null | undefined }> } | null | undefined };

export type GetSkillsQueryVariables = Exact<{ [key: string]: never; }>;


export type GetSkillsQuery = { __typename?: 'Query', skills: Array<{ __typename?: 'Skill', name: string, description?: string | null | undefined, enabled: boolean, path?: string | null | undefined }> };

export type GetConfigQueryVariables = Exact<{ [key: string]: never; }>;


export type GetConfigQuery = { __typename?: 'Query', config?: { __typename?: 'AppConfig', wizardCompleted?: boolean | null | undefined, agent?: { __typename?: 'AgentConfig', name?: string | null | undefined, systemPrompt?: string | null | undefined, provider?: string | null | undefined, model?: string | null | undefined, apiKey?: string | null | undefined, baseURL?: string | null | undefined, ollamaHost?: string | null | undefined, ollamaApiKey?: string | null | undefined, anthropicApiKey?: string | null | undefined, dockerModelRunnerEndpoint?: string | null | undefined, dockerModelRunnerModel?: string | null | undefined } | null | undefined, capabilities?: { __typename?: 'CapabilitiesConfig', browser?: boolean | null | undefined, terminal?: boolean | null | undefined, subagents?: boolean | null | undefined, memory?: boolean | null | undefined, mcp?: boolean | null | undefined, filesystem?: boolean | null | undefined, sessions?: boolean | null | undefined } | null | undefined, database?: { __typename?: 'DatabaseConfig', driver?: string | null | undefined, dsn?: string | null | undefined, maxOpenConns?: number | null | undefined, maxIdleConns?: number | null | undefined } | null | undefined, memory?: { __typename?: 'MemoryConfig', backend?: string | null | undefined, filePath?: string | null | undefined, neo4j?: { __typename?: 'Neo4jConfig', uri?: string | null | undefined, user?: string | null | undefined, password?: string | null | undefined } | null | undefined } | null | undefined, subagents?: { __typename?: 'SubagentsConfig', maxConcurrent?: number | null | undefined, defaultTimeout?: string | null | undefined } | null | undefined, graphql?: { __typename?: 'GraphQLConfig', enabled?: boolean | null | undefined, port?: number | null | undefined, host?: string | null | undefined, baseUrl?: string | null | undefined } | null | undefined, logging?: { __typename?: 'LoggingConfig', level?: string | null | undefined, path?: string | null | undefined } | null | undefined, secrets?: { __typename?: 'SecretsConfig', backend?: string | null | undefined, file?: { __typename?: 'FileSecretsConfig', path?: string | null | undefined } | null | undefined, openbao?: { __typename?: 'OpenbaoSecretsConfig', url?: string | null | undefined, token?: string | null | undefined } | null | undefined } | null | undefined, scheduler?: { __typename?: 'SchedulerConfig', enabled?: boolean | null | undefined, memoryEnabled?: boolean | null | undefined, memoryInterval?: string | null | undefined } | null | undefined, activeSessions: Array<{ __typename?: 'ActiveSession', id: string, address?: string | null | undefined, status?: string | null | undefined, channel?: string | null | undefined, user?: string | null | undefined }>, channels: Array<{ __typename?: 'ChannelConfig', channelId: string, channelName?: string | null | undefined, enabled: boolean }>, channelSecrets?: { __typename?: 'ChannelSecretsConfig', telegramEnabled?: boolean | null | undefined, telegramToken?: string | null | undefined, discordEnabled?: boolean | null | undefined, discordToken?: string | null | undefined, whatsAppEnabled?: boolean | null | undefined, whatsAppPhoneId?: string | null | undefined, whatsAppApiToken?: string | null | undefined, twilioEnabled?: boolean | null | undefined, twilioAccountSid?: string | null | undefined, twilioAuthToken?: string | null | undefined, twilioFromNumber?: string | null | undefined } | null | undefined } | null | undefined };

export type GetSystemFilesQueryVariables = Exact<{ [key: string]: never; }>;


export type GetSystemFilesQuery = { __typename?: 'Query', systemFiles: Array<{ __typename?: 'SystemFile', name: string, content?: string | null | undefined }> };

export type OnMessageReceivedSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type OnMessageReceivedSubscription = { __typename?: 'Subscription', onMessageReceived?: { __typename?: 'EventPayload', type: string, timestamp: string, data?: Record<string, unknown> | null | undefined } | null | undefined };


export const SendMessageDocument = gql`
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
export const AddTaskDocument = gql`
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
export const CompleteTaskDocument = gql`
    mutation CompleteTask($taskId: String!) {
  completeTask(taskId: $taskId)
}
    `;
export const RemoveTaskDocument = gql`
    mutation RemoveTask($taskId: String!) {
  removeTask(taskId: $taskId)
}
    `;
export const ToggleTaskDocument = gql`
    mutation ToggleTask($id: String!, $enabled: Boolean!) {
  toggleTask(id: $id, enabled: $enabled) {
    success
    id
    enabled
  }
}
    `;
export const UpdateTaskDocument = gql`
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
export const ConnectMcpDocument = gql`
    mutation ConnectMcp($name: String!, $transport: String!, $url: String) {
  connectMcp(name: $name, transport: $transport, url: $url) {
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
export const DisconnectMcpDocument = gql`
    mutation DisconnectMcp($name: String!) {
  disconnectMcp(name: $name)
}
    `;
export const InitiateOAuthDocument = gql`
    mutation InitiateOAuth($name: String!, $url: String!) {
  initiateOAuth(name: $name, url: $url) {
    success
    authUrl
    error
  }
}
    `;
export const AddMemoryNodeDocument = gql`
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
export const UpdateMemoryNodeDocument = gql`
    mutation UpdateMemoryNode($id: String!, $label: String!, $type: String!, $value: String!, $properties: String) {
  updateMemoryNode(
    id: $id
    label: $label
    type: $type
    value: $value
    properties: $properties
  ) {
    id
    label
    type
    value
    createdAt
  }
}
    `;
export const DeleteMemoryNodeDocument = gql`
    mutation DeleteMemoryNode($id: String!) {
  deleteMemoryNode(id: $id)
}
    `;
export const EnableSkillDocument = gql`
    mutation EnableSkill($name: String!) {
  enableSkill(name: $name)
}
    `;
export const DisableSkillDocument = gql`
    mutation DisableSkill($name: String!) {
  disableSkill(name: $name)
}
    `;
export const DeleteSkillDocument = gql`
    mutation DeleteSkill($name: String!) {
  deleteSkill(name: $name)
}
    `;
export const ImportSkillDocument = gql`
    mutation ImportSkill($data: String!) {
  importSkill(data: $data) {
    success
    error
  }
}
    `;
export const SetToolPermissionDocument = gql`
    mutation SetToolPermission($userId: String!, $toolName: String!, $mode: String!) {
  setToolPermission(userId: $userId, toolName: $toolName, mode: $mode) {
    success
    error
  }
}
    `;
export const DeleteToolPermissionDocument = gql`
    mutation DeleteToolPermission($userId: String!, $toolName: String!) {
  deleteToolPermission(userId: $userId, toolName: $toolName) {
    success
    error
  }
}
    `;
export const SetAllToolPermissionsDocument = gql`
    mutation SetAllToolPermissions($userId: String!, $mode: String!) {
  setAllToolPermissions(userId: $userId, mode: $mode) {
    success
    error
  }
}
    `;
export const DeleteUserDocument = gql`
    mutation DeleteUser($conversationId: String!) {
  deleteUser(conversationId: $conversationId) {
    success
    error
  }
}
    `;
export const UpdateConfigDocument = gql`
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
export const WriteSystemFileDocument = gql`
    mutation WriteSystemFile($name: String!, $content: String!) {
  writeSystemFile(name: $name, content: $content) {
    success
    error
  }
}
    `;
export const GetAgentDocument = gql`
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
export const GetMetricsDocument = gql`
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
export const GetChannelsDocument = gql`
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
export const GetConversationsDocument = gql`
    query GetConversations {
  conversations {
    id
    channelId
    channelName
    isGroup
    participantId
    participantName
    lastMessageAt
    unreadCount
  }
}
    `;
export const GetMessagesDocument = gql`
    query GetMessages($conversationId: String!) {
  messages(conversationId: $conversationId) {
    id
    conversationId
    role
    content
    createdAt
  }
}
    `;
export const GetTasksDocument = gql`
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
export const GetMcpServersDocument = gql`
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
export const GetMcpToolsDocument = gql`
    query GetMcpTools {
  mcpTools {
    name
    serverName
    description
  }
}
    `;
export const GetMcpUsersDocument = gql`
    query GetMcpUsers {
  mcpUsers {
    channelId
    displayName
    isAgent
  }
}
    `;
export const GetToolPermissionsDocument = gql`
    query GetToolPermissions($userId: String!) {
  toolPermissions(userId: $userId) {
    toolName
    mode
  }
}
    `;
export const GetMemoryDocument = gql`
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
export const GetSkillsDocument = gql`
    query GetSkills {
  skills {
    name
    description
    enabled
    path
  }
}
    `;
export const GetConfigDocument = gql`
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
    }
    wizardCompleted
  }
}
    `;
export const GetSystemFilesDocument = gql`
    query GetSystemFiles {
  systemFiles {
    name
    content
  }
}
    `;
export const OnMessageReceivedDocument = gql`
    subscription OnMessageReceived {
  onMessageReceived {
    type
    timestamp
    data
  }
}
    `;

export type SdkFunctionWrapper = <T>(action: (requestHeaders?:Record<string, string>) => Promise<T>, operationName: string, operationType?: string, variables?: any) => Promise<T>;


const defaultWrapper: SdkFunctionWrapper = (action, _operationName, _operationType, _variables) => action();

export function getSdk(client: GraphQLClient, withWrapper: SdkFunctionWrapper = defaultWrapper) {
  return {
    SendMessage(variables: SendMessageMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SendMessageMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SendMessageMutation>({ document: SendMessageDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SendMessage', 'mutation', variables);
    },
    AddTask(variables: AddTaskMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<AddTaskMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<AddTaskMutation>({ document: AddTaskDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'AddTask', 'mutation', variables);
    },
    CompleteTask(variables: CompleteTaskMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<CompleteTaskMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<CompleteTaskMutation>({ document: CompleteTaskDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'CompleteTask', 'mutation', variables);
    },
    RemoveTask(variables: RemoveTaskMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<RemoveTaskMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<RemoveTaskMutation>({ document: RemoveTaskDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'RemoveTask', 'mutation', variables);
    },
    ToggleTask(variables: ToggleTaskMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ToggleTaskMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ToggleTaskMutation>({ document: ToggleTaskDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ToggleTask', 'mutation', variables);
    },
    UpdateTask(variables: UpdateTaskMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<UpdateTaskMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<UpdateTaskMutation>({ document: UpdateTaskDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'UpdateTask', 'mutation', variables);
    },
    ConnectMcp(variables: ConnectMcpMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ConnectMcpMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ConnectMcpMutation>({ document: ConnectMcpDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ConnectMcp', 'mutation', variables);
    },
    DisconnectMcp(variables: DisconnectMcpMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DisconnectMcpMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DisconnectMcpMutation>({ document: DisconnectMcpDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DisconnectMcp', 'mutation', variables);
    },
    InitiateOAuth(variables: InitiateOAuthMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<InitiateOAuthMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<InitiateOAuthMutation>({ document: InitiateOAuthDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'InitiateOAuth', 'mutation', variables);
    },
    AddMemoryNode(variables: AddMemoryNodeMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<AddMemoryNodeMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<AddMemoryNodeMutation>({ document: AddMemoryNodeDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'AddMemoryNode', 'mutation', variables);
    },
    UpdateMemoryNode(variables: UpdateMemoryNodeMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<UpdateMemoryNodeMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<UpdateMemoryNodeMutation>({ document: UpdateMemoryNodeDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'UpdateMemoryNode', 'mutation', variables);
    },
    DeleteMemoryNode(variables: DeleteMemoryNodeMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteMemoryNodeMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteMemoryNodeMutation>({ document: DeleteMemoryNodeDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteMemoryNode', 'mutation', variables);
    },
    EnableSkill(variables: EnableSkillMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<EnableSkillMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<EnableSkillMutation>({ document: EnableSkillDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'EnableSkill', 'mutation', variables);
    },
    DisableSkill(variables: DisableSkillMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DisableSkillMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DisableSkillMutation>({ document: DisableSkillDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DisableSkill', 'mutation', variables);
    },
    DeleteSkill(variables: DeleteSkillMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteSkillMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteSkillMutation>({ document: DeleteSkillDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteSkill', 'mutation', variables);
    },
    ImportSkill(variables: ImportSkillMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<ImportSkillMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<ImportSkillMutation>({ document: ImportSkillDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'ImportSkill', 'mutation', variables);
    },
    SetToolPermission(variables: SetToolPermissionMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SetToolPermissionMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SetToolPermissionMutation>({ document: SetToolPermissionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SetToolPermission', 'mutation', variables);
    },
    DeleteToolPermission(variables: DeleteToolPermissionMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteToolPermissionMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteToolPermissionMutation>({ document: DeleteToolPermissionDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteToolPermission', 'mutation', variables);
    },
    SetAllToolPermissions(variables: SetAllToolPermissionsMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<SetAllToolPermissionsMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<SetAllToolPermissionsMutation>({ document: SetAllToolPermissionsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'SetAllToolPermissions', 'mutation', variables);
    },
    DeleteUser(variables: DeleteUserMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<DeleteUserMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<DeleteUserMutation>({ document: DeleteUserDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'DeleteUser', 'mutation', variables);
    },
    UpdateConfig(variables: UpdateConfigMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<UpdateConfigMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<UpdateConfigMutation>({ document: UpdateConfigDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'UpdateConfig', 'mutation', variables);
    },
    WriteSystemFile(variables: WriteSystemFileMutationVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<WriteSystemFileMutation> {
      return withWrapper((wrappedRequestHeaders) => client.request<WriteSystemFileMutation>({ document: WriteSystemFileDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'WriteSystemFile', 'mutation', variables);
    },
    GetAgent(variables?: GetAgentQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetAgentQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetAgentQuery>({ document: GetAgentDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetAgent', 'query', variables);
    },
    GetMetrics(variables?: GetMetricsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMetricsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMetricsQuery>({ document: GetMetricsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMetrics', 'query', variables);
    },
    GetChannels(variables?: GetChannelsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetChannelsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetChannelsQuery>({ document: GetChannelsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetChannels', 'query', variables);
    },
    GetConversations(variables?: GetConversationsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetConversationsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetConversationsQuery>({ document: GetConversationsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetConversations', 'query', variables);
    },
    GetMessages(variables: GetMessagesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMessagesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMessagesQuery>({ document: GetMessagesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMessages', 'query', variables);
    },
    GetTasks(variables?: GetTasksQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetTasksQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetTasksQuery>({ document: GetTasksDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetTasks', 'query', variables);
    },
    GetMcpServers(variables?: GetMcpServersQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMcpServersQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMcpServersQuery>({ document: GetMcpServersDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMcpServers', 'query', variables);
    },
    GetMcpTools(variables?: GetMcpToolsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMcpToolsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMcpToolsQuery>({ document: GetMcpToolsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMcpTools', 'query', variables);
    },
    GetMcpUsers(variables?: GetMcpUsersQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMcpUsersQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMcpUsersQuery>({ document: GetMcpUsersDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMcpUsers', 'query', variables);
    },
    GetToolPermissions(variables: GetToolPermissionsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetToolPermissionsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetToolPermissionsQuery>({ document: GetToolPermissionsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetToolPermissions', 'query', variables);
    },
    GetMemory(variables?: GetMemoryQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetMemoryQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetMemoryQuery>({ document: GetMemoryDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetMemory', 'query', variables);
    },
    GetSkills(variables?: GetSkillsQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetSkillsQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetSkillsQuery>({ document: GetSkillsDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetSkills', 'query', variables);
    },
    GetConfig(variables?: GetConfigQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetConfigQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetConfigQuery>({ document: GetConfigDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetConfig', 'query', variables);
    },
    GetSystemFiles(variables?: GetSystemFilesQueryVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<GetSystemFilesQuery> {
      return withWrapper((wrappedRequestHeaders) => client.request<GetSystemFilesQuery>({ document: GetSystemFilesDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'GetSystemFiles', 'query', variables);
    },
    OnMessageReceived(variables?: OnMessageReceivedSubscriptionVariables, requestHeaders?: GraphQLClientRequestHeaders, signal?: RequestInit['signal']): Promise<OnMessageReceivedSubscription> {
      return withWrapper((wrappedRequestHeaders) => client.request<OnMessageReceivedSubscription>({ document: OnMessageReceivedDocument, variables, requestHeaders: { ...requestHeaders, ...wrappedRequestHeaders }, signal }), 'OnMessageReceived', 'subscription', variables);
    }
  };
}
export type Sdk = ReturnType<typeof getSdk>;