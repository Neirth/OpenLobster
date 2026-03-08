// Package services provides domain services. Types and constructors are
// re-exported from subpackages for backward compatibility.
package services

import (
	svccompaction "github.com/neirth/openlobster/internal/domain/services/compaction"
	svccontext "github.com/neirth/openlobster/internal/domain/services/context_builder"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
	svcmsgcompaction "github.com/neirth/openlobster/internal/domain/services/message_compaction"
	svcmsgprocessor "github.com/neirth/openlobster/internal/domain/services/message_processor"
	svcpairing "github.com/neirth/openlobster/internal/domain/services/pairing"
	svcscheduler "github.com/neirth/openlobster/internal/domain/services/scheduler"
	svcsubagent "github.com/neirth/openlobster/internal/domain/services/subagent"
)

// Compaction (legacy API)
type CompactionService = svccompaction.Service

var NewCompactionService = svccompaction.NewService

// Pairing
type PairingService = svcpairing.Service

var NewPairingService = svcpairing.NewService

const (
	PairingStatusPending  = svcpairing.StatusPending
	PairingStatusApproved = svcpairing.StatusApproved
	PairingStatusExpired  = svcpairing.StatusExpired
	PairingStatusDenied   = svcpairing.StatusDenied
)

// Scheduler
type Scheduler = svcscheduler.Scheduler

var NewScheduler = svcscheduler.NewScheduler

const MemoryConsolidationPrompt = svcscheduler.MemoryConsolidationPrompt

// SubAgent
type SubAgentService = svcsubagent.Service

var NewSubAgentService = svcsubagent.NewService

// Dashboard Query/Command
type DashboardQueryService = svcdashboard.QueryService
type DashboardCommandService = svcdashboard.CommandService

var NewDashboardQueryService = svcdashboard.NewQueryService
var NewDashboardCommandService = svcdashboard.NewCommandService

type PortsGraph = svcdashboard.PortsGraph
type PortsGraphResult = svcdashboard.PortsGraphResult

// Message compaction
type MessageCompactionService = svcmsgcompaction.Service

var NewMessageCompactionService = svcmsgcompaction.NewService

// Message processor and event bus
type EventBus = svcmsgprocessor.EventBus
type DefaultEventBus = svcmsgprocessor.DefaultEventBus
type EventHandler = svcmsgprocessor.EventHandler
type MessageProcessorService = svcmsgprocessor.MessageProcessorService
type PromptBuilderService = svcmsgprocessor.PromptBuilderService
type ValidationError = svcmsgprocessor.ValidationError

var NewEventBus = svcmsgprocessor.NewEventBus
var NewMessageProcessorService = svcmsgprocessor.NewMessageProcessorService
var NewPromptBuilderService = svcmsgprocessor.NewPromptBuilderService
var NewPromptBuilderServiceWithContext = svcmsgprocessor.NewPromptBuilderServiceWithContext
var ErrEmptyChannel = svcmsgprocessor.ErrEmptyChannel

// Context builder
type ContextBuilderService = svccontext.Service
type MemoryDigestService = svccontext.MemoryDigestService
type MemoryDigestCache = svccontext.MemoryDigestCache

var NewContextBuilderService = svccontext.NewService
var NewMemoryDigestService = svccontext.NewMemoryDigestService
