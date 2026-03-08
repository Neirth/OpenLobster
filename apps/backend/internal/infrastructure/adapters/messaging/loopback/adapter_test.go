package loopback

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	a := New()
	assert.NotNil(t, a)
}

func TestAdapter_SendMessage(t *testing.T) {
	a := New()
	err := a.SendMessage(context.Background(), &models.Message{Content: "test"})
	assert.NoError(t, err)
}

func TestAdapter_SendMedia(t *testing.T) {
	a := New()
	err := a.SendMedia(context.Background(), &ports.Media{})
	assert.NoError(t, err)
}

func TestAdapter_HandleWebhook(t *testing.T) {
	a := New()
	msg, err := a.HandleWebhook(context.Background(), []byte("{}"))
	assert.NoError(t, err)
	assert.Nil(t, msg)
}

func TestAdapter_GetUserInfo(t *testing.T) {
	a := New()
	info, err := a.GetUserInfo(context.Background(), "user1")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "user1", info.ID)
	assert.Equal(t, "loopback", info.Username)
	assert.Equal(t, "Loopback", info.DisplayName)
}

func TestAdapter_React(t *testing.T) {
	a := New()
	err := a.React(context.Background(), "msg1", "👍")
	assert.NoError(t, err)
}

func TestAdapter_GetCapabilities(t *testing.T) {
	a := New()
	caps := a.GetCapabilities()
	assert.True(t, caps.HasTextStream)
}

func TestAdapter_Start(t *testing.T) {
	a := New()
	err := a.Start(context.Background(), func(ctx context.Context, m *models.Message) {})
	assert.NoError(t, err)
}

func TestAdapter_ConvertAudioForPlatform(t *testing.T) {
	a := New()
	data := []byte{1, 2, 3}
	out, fmt, err := a.ConvertAudioForPlatform(context.Background(), data, "ogg")
	assert.NoError(t, err)
	assert.Equal(t, data, out)
	assert.Equal(t, "ogg", fmt)
}
