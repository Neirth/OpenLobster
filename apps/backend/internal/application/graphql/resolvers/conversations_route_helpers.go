// Copyright (c) OpenLobster contributors. See LICENSE for details.

package resolvers

import (
	"context"
	"strings"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
)

type lastChannelForUserResolver interface {
	GetLastChannelForUser(ctx context.Context, userID string) (channelType, platformChannelID string, err error)
}

type lastChannelForUserByChannelResolver interface {
	GetLastChannelForUserByChannel(ctx context.Context, userID, channelType string) (resolvedChannelType, platformChannelID string, err error)
}

func findConversationRoute(convs []dto.ConversationSnapshot, conversationID string) (dto.ConversationSnapshot, bool) {
	target := strings.TrimSpace(conversationID)
	if target == "" {
		return dto.ConversationSnapshot{}, false
	}

	for _, conv := range convs {
		if strings.TrimSpace(conv.ID) == target || strings.TrimSpace(conv.ChannelID) == target {
			return conv, true
		}
	}

	return dto.ConversationSnapshot{}, false
}

func resolveConversationRouteChannel(
	ctx context.Context,
	deps *Deps,
	conv dto.ConversationSnapshot,
	currentChannelID,
	currentChannelType string,
) (string, string) {
	if deps == nil || deps.UserChannelRepo == nil || conv.IsGroup {
		return currentChannelID, currentChannelType
	}

	participantID := strings.TrimSpace(conv.ParticipantID)
	if participantID == "" {
		return currentChannelID, currentChannelType
	}

	fallbackChannelType := strings.TrimSpace(currentChannelType)
	if fallbackChannelType == "" {
		fallbackChannelType = strings.TrimSpace(conv.ChannelType)
	}

	if scopedRepo, ok := deps.UserChannelRepo.(lastChannelForUserByChannelResolver); ok && fallbackChannelType != "" {
		if routeChannelType, routeChannelID, err := scopedRepo.GetLastChannelForUserByChannel(ctx, participantID, fallbackChannelType); err == nil {
			if isValidConversationRouteChannel(routeChannelID) {
				if strings.TrimSpace(routeChannelType) == "" {
					routeChannelType = fallbackChannelType
				}
				return strings.TrimSpace(routeChannelID), strings.TrimSpace(routeChannelType)
			}
		}
	}

	if genericRepo, ok := deps.UserChannelRepo.(lastChannelForUserResolver); ok {
		if routeChannelType, routeChannelID, err := genericRepo.GetLastChannelForUser(ctx, participantID); err == nil {
			if isValidConversationRouteChannel(routeChannelID) {
				if strings.TrimSpace(routeChannelType) == "" {
					routeChannelType = fallbackChannelType
				}
				return strings.TrimSpace(routeChannelID), strings.TrimSpace(routeChannelType)
			}
		}
	}

	return currentChannelID, currentChannelType
}

func isValidConversationRouteChannel(channelID string) bool {
	normalized := strings.TrimSpace(channelID)
	if normalized == "" {
		return false
	}

	switch strings.ToLower(normalized) {
	case "dashboard", "agent":
		return false
	default:
		return true
	}
}
