package serve

import (
	"context"
	"log"

	"github.com/neirth/openlobster/internal/domain/repositories"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
)

// initDatabase opens the database, runs migrations and instantiates every
// repository. Also loads tool-permission overrides from config and DB.
func (a *App) initDatabase() {
	cfg := a.Cfg

	db, err := persistence.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	a.db = db

	if err := persistence.Migrate(db.GormDB(), cfg.Database.Driver); err != nil {
		log.Fatalf("failed to migrate database schema: %v", err)
	}
	log.Println("database schema up to date")

	gormDB := db.GormDB()
	a.TaskRepo = repositories.NewTaskRepository(gormDB)
	a.MessageRepo = repositories.NewMessageRepository(gormDB)
	a.SessionRepo = repositories.NewSessionRepository(gormDB)
	a.UserRepo = repositories.NewUserRepository(gormDB)
	a.ConvRepo = repositories.NewConversationRepository(gormDB)
	a.DashMsgRepo = repositories.NewDashboardMessageRepository(a.MessageRepo)
	a.ToolPermRepo = repositories.NewToolPermissionRepository(gormDB)
	a.MCPServerRepo = repositories.NewMCPServerRepository(gormDB)
	a.PairingRepo = repositories.NewPairingRepository(gormDB)
	a.UserChannelRepo = repositories.NewUserChannelRepository(gormDB)
}

// loadPermissions loads tool-permission overrides from config and DB into permManager.
// Called from initServices after the permission manager is created.
func (a *App) loadPermissions(permManager *permissions.Manager) {
	cfg := a.Cfg

	for toolName, permCfg := range cfg.Permissions.ToolPermissions {
		userID := "*"
		if permCfg.User != "" {
			userID = permCfg.User
		}
		if permCfg.Mode == "deny" {
			permManager.SetPermission(userID, toolName, permissions.PermissionDeny)
		} else {
			permManager.SetPermission(userID, toolName, permissions.PermissionAlways)
		}
	}
	if len(cfg.Permissions.ToolPermissions) > 0 {
		log.Printf("permissions: loaded %d global entries from config", len(cfg.Permissions.ToolPermissions))
	}

	if savedPerms, err := a.ToolPermRepo.ListAll(context.Background()); err == nil {
		for _, p := range savedPerms {
			if p.Mode == "allow" {
				permManager.SetPermission(p.UserID, p.ToolName, permissions.PermissionAlways)
			} else {
				permManager.SetPermission(p.UserID, p.ToolName, permissions.PermissionDeny)
			}
		}
		if len(savedPerms) > 0 {
			log.Printf("permissions: loaded %d entries from database", len(savedPerms))
		}
	} else {
		log.Printf("permissions: failed to load from database: %v", err)
	}
}
