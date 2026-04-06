package serve

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	domainservices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/memory_consolidation"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
)

// startAndWait starts all background goroutines (scheduler, plugin messaging
// loops, HTTP server) and blocks until SIGINT/SIGTERM, then performs a
// graceful shutdown.
func (a *App) startAndWait() {
	ctx, cancel := context.WithCancel(context.Background())
	a.Ctx = ctx
	a.Cancel = cancel
	a.ChannelStartCtx = ctx
	defer cancel()

	// Ensure workspace exists.
	if err := os.MkdirAll(a.Cfg.Workspace.Path, 0755); err != nil {
		log.Printf("lifecycle: failed to create workspace: %v", err)
	}

	// Scheduler
	if a.Cfg.Scheduler.Enabled {
		dispatcher := domainhandlers.NewLoopbackDispatcher(a.MsgHandler)
		consolidationSvc := memory_consolidation.NewService(
			a.MessageRepo,
			a.MemoryAdapter,
			a.AIProvider,
			a.UserRepo,
			a.SessionRepo,
			a.ToolRegistry,
		)
		sched := domainservices.NewScheduler(
			a.Cfg.Scheduler.MemoryInterval,
			a.Cfg.Scheduler.MemoryEnabled,
			dispatcher,
			a.TaskRepo,
			consolidationSvc,
		)
		a.SchedulerNotify = sched.Notify
		a.SchedulerUpdateMemoryInterval = sched.UpdateMemoryInterval
		go sched.Run(ctx)
	}

	// Change working directory to workspace so tools (terminal, etc.) operate there.
	if err := os.Chdir(a.Cfg.Workspace.Path); err != nil {
		log.Printf("lifecycle: failed to chdir to workspace: %v", err)
	} else {
		log.Printf("lifecycle: changed working directory to %s", a.Cfg.Workspace.Path)
	}

	// Rebuild and start messaging adapters (plugins and native) in a single
	// reconciled runtime path so hot-reloads use the same wiring rules as boot.
	a.rebuildMessagingRuntime()

	// HTTP server
	addr := a.HTTPServer.Addr
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("openlobster listening on http://%s", addr)
	go func() {
		if err := a.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-sig
	log.Println("shutting down…")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := a.HTTPServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}

	// Stop messaging runtime before closing the plugin registry so dedicated
	// loop runners are terminated and do not remain orphaned after shutdown.
	a.stopMessagingRuntime()

	if a.PluginRegistry != nil {
		a.PluginRegistry.Close()
	}

	if gml, ok := a.MemoryAdapter.(interface{ Close() error }); ok {
		if err := gml.Close(); err != nil {
			log.Printf("memory backend flush error: %v", err)
		} else {
			log.Println("memory backend: flushed to disk")
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}
	if err := logging.Close(); err != nil {
		log.Printf("logging close error: %v", err)
	}
}
