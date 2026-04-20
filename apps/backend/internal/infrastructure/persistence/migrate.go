// Copyright (c) OpenLobster contributors. See LICENSE for details.

package persistence

import (
	"fmt"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"gorm.io/gorm"
)

// Migrate brings the database schema up to date.
//
// AutoMigrate handles tables and indexes declared via GORM struct tags.
// It is additive-only: it creates missing tables/columns/indexes but never
// drops or renames anything, so it is safe to run on every startup.
//
// SQL views cannot be expressed as GORM struct tags, so they are managed
// here with DROP + CREATE. This ensures the view definition always reflects
// the latest code on every restart without a separate migration step.
//
// driver is the configured database driver ("sqlite", "postgres", "mysql");
// it is used to choose driver-specific SQL fragments where needed.
func Migrate(db *gorm.DB, driver string) error {
	// ── 1. Tables + indexes (all declared via GORM struct tags) ───────────────
	if err := db.AutoMigrate(
		&domainmodels.UserModel{},
		&domainmodels.ChannelModel{},
		&domainmodels.GroupModel{},
		&domainmodels.GroupUserModel{},
		&domainmodels.UserChannelModel{}, // Restored UserChannelModel per user's request
		&domainmodels.ConversationModel{},
		&domainmodels.MessageModel{},
		&domainmodels.MessageAttachmentModel{},
		&domainmodels.TaskModel{},
		&domainmodels.PairingModel{},
		&domainmodels.MCPServerModel{},
		&domainmodels.ToolPermissionModel{},
	); err != nil {
		return fmt.Errorf("AutoMigrate: %w", err)
	}

	// Ensure the reserved loopback user exists so foreign keys from tool_permissions
	// can safely reference it when configuring scheduled-task agent permissions.
	var loopbackCount int64
	if err := db.Model(&domainmodels.UserModel{}).
		Where("id = ?", "loopback").
		Count(&loopbackCount).Error; err != nil {
		return fmt.Errorf("check loopback user: %w", err)
	}
	if loopbackCount == 0 {
		now := time.Now().UTC()
		loopbackUser := &domainmodels.UserModel{
			ID:        "loopback",
			PrimaryID: "loopback",
			Name:      "Loopback",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.Create(loopbackUser).Error; err != nil {
			return fmt.Errorf("seed loopback user: %w", err)
		}
	}

	// ── 2. Views ─────────────────────────────────────────────────────────────
	// Drop in reverse dependency order. We always Drop before Create because
	// SQLite does not support CREATE OR REPLACE VIEW.
	if err := db.Migrator().DropView("v_conversation_summary"); err != nil {
		return fmt.Errorf("drop view v_conversation_summary: %w", err)
	}
	if err := db.Migrator().DropView("v_active_conversations"); err != nil {
		return fmt.Errorf("drop view v_active_conversations: %w", err)
	}
	// Drop legacy v_user_display if it still exists from older installs.
	_ = db.Migrator().DropView("v_user_display")

	// v_active_conversations — pre-filtered to is_active rows.
	qActiveConvs := db.Model(&domainmodels.ConversationModel{}).
		Where("is_active = ?", true).
		Select("id, channel_id, group_id, user_id, model_id, is_active, started_at, updated_at")
	if err := db.Migrator().CreateView("v_active_conversations", gorm.ViewOption{Query: qActiveConvs}); err != nil {
		return fmt.Errorf("create view v_active_conversations: %w", err)
	}

	// v_conversation_summary — CTE + joins to other views; too complex for
	// GORM's query builder, so we use raw SQL with driver-specific date formatting.
	if err := db.Exec(conversationSummaryView(driver)).Error; err != nil {
		return fmt.Errorf("create view v_conversation_summary: %w", err)
	}

	return nil
}

// conversationSummaryView returns the CREATE VIEW statement for
// v_conversation_summary, using driver-appropriate date formatting.
func conversationSummaryView(driver string) string {
	// SQLite stores timestamps as text; datetime() normalises them.
	// Postgres/MySQL timestamps are native types — cast to text for consistency.
	var lastMsgExpr string
	switch driver {
	case "postgres", "pgx":
		lastMsgExpr = "COALESCE(ms.last_message_at::text, c.started_at::text, '')"
	case "mysql":
		lastMsgExpr = "COALESCE(DATE_FORMAT(ms.last_message_at,'%Y-%m-%d %H:%i:%s'), DATE_FORMAT(c.started_at,'%Y-%m-%d %H:%i:%s'), '')"
	default: // sqlite
		lastMsgExpr = "COALESCE(datetime(ms.last_message_at), datetime(c.started_at), '')"
	}

	return fmt.Sprintf(`CREATE VIEW v_conversation_summary AS
	WITH msg_stats AS (
	    SELECT conversation_id,
	           MAX(created_at)                                        AS last_message_at,
	           SUM(CASE WHEN role = 'user' THEN 1 ELSE 0 END)        AS unread_count
	    FROM messages
	    GROUP BY conversation_id
	)
	SELECT
	    c.id,
	    c.channel_id,
	    COALESCE(ch.type, '')                                         AS channel_type,
	    COALESCE(g.name, ch.type, c.channel_id, '')                  AS channel_name,
	    COALESCE(g.name, '')                                         AS group_name,
	    CASE WHEN c.group_id IS NOT NULL THEN 1 ELSE 0 END           AS is_group,
	    COALESCE(c.user_id, '')                                       AS participant_id,
		COALESCE(NULLIF(u.name,''), c.user_id, '') AS participant_name,
	    %s                                                            AS last_message_at,
	    COALESCE(ms.unread_count, 0)                                  AS unread_count
	FROM conversations c
	LEFT JOIN channels       ch ON ch.id    = c.channel_id
	LEFT JOIN groups          g ON g.id     = c.group_id
	LEFT JOIN users           u ON u.id     = c.user_id
	LEFT JOIN msg_stats       ms ON ms.conversation_id = c.id
	WHERE COALESCE(c.channel_id, '') != 'loopback'
	  AND COALESCE(ch.type, '') != 'loopback'
	ORDER BY last_message_at DESC`, lastMsgExpr)
}
