// Copyright (c) OpenLobster contributors. See LICENSE for details.

package mappers

import (
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentsToGenerated_Empty(t *testing.T) {
	result := attachmentsToGenerated(nil)
	assert.Nil(t, result)

	result = attachmentsToGenerated([]dto.AttachmentSnapshot{})
	assert.Nil(t, result)
}

func TestAttachmentsToGenerated_ImageAttachment(t *testing.T) {
	snapshots := []dto.AttachmentSnapshot{
		{Type: "image", URL: "https://example.com/img.jpg", MIMEType: "image/jpeg"},
	}
	result := attachmentsToGenerated(snapshots)

	require.Len(t, result, 1)
	assert.Equal(t, "image", result[0].Type)
	require.NotNil(t, result[0].URL)
	assert.Equal(t, "https://example.com/img.jpg", *result[0].URL)
	require.NotNil(t, result[0].MimeType)
	assert.Equal(t, "image/jpeg", *result[0].MimeType)
	assert.Nil(t, result[0].Filename)
}

func TestAttachmentsToGenerated_WithFilename(t *testing.T) {
	snapshots := []dto.AttachmentSnapshot{
		{Type: "document", Filename: "report.pdf", MIMEType: "application/pdf"},
	}
	result := attachmentsToGenerated(snapshots)

	require.Len(t, result, 1)
	assert.Equal(t, "document", result[0].Type)
	require.NotNil(t, result[0].Filename)
	assert.Equal(t, "report.pdf", *result[0].Filename)
	assert.Nil(t, result[0].URL)
}

func TestMessagesToGenerated_ExposesAttachments(t *testing.T) {
	list := []dto.MessageSnapshot{
		{
			ID:             "msg-1",
			ConversationID: "conv-1",
			Role:           "user",
			Content:        "check this image",
			CreatedAt:      "2024-01-01T00:00:00Z",
			Attachments: []dto.AttachmentSnapshot{
				{Type: "image", URL: "https://example.com/photo.jpg", MIMEType: "image/jpeg"},
			},
		},
	}
	result := MessagesToGenerated(list)

	require.Len(t, result, 1)
	require.Len(t, result[0].Attachments, 1)
	assert.Equal(t, "image", result[0].Attachments[0].Type)
	require.NotNil(t, result[0].Attachments[0].URL)
	assert.Equal(t, "https://example.com/photo.jpg", *result[0].Attachments[0].URL)
}

func TestMessagesToGenerated_NoAttachments(t *testing.T) {
	list := []dto.MessageSnapshot{
		{
			ID:             "msg-1",
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "hello",
			CreatedAt:      "2024-01-01T00:00:00Z",
		},
	}
	result := MessagesToGenerated(list)

	require.Len(t, result, 1)
	assert.Nil(t, result[0].Attachments)
}

func TestMessagesToGenerated_SkipsCompaction(t *testing.T) {
	list := []dto.MessageSnapshot{
		{Role: "compaction", Content: "..."},
		{ID: "msg-1", ConversationID: "c", Role: "user", Content: "hi", CreatedAt: "now"},
	}
	result := MessagesToGenerated(list)

	require.Len(t, result, 1)
	assert.Equal(t, "hi", result[0].Content)
}
