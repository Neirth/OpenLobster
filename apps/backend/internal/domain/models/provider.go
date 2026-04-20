package models

type ModelProvider string

const (
	ProviderOpenRouter ModelProvider = "openrouter"
	ProviderOllama     ModelProvider = "ollama"
	ProviderOpenCode   ModelProvider = "opencode"
	ProviderOpenAI     ModelProvider = "openai"
)

type MemoryBackendType string

const (
	MemoryFile  MemoryBackendType = "file"
	MemoryGML   MemoryBackendType = "gml"
	MemoryNeo4j MemoryBackendType = "neo4j"
)
