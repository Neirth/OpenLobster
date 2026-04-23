// Copyright (c) OpenLobster contributors. See LICENSE for details.

package serve

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type blockingMessagingAdapter struct {
	entered    chan struct{}
	proceed    <-chan struct{}
	startErr   error
	startCalls atomic.Int32
}

func (b *blockingMessagingAdapter) SendMessage(context.Context, *models.Message) error { return nil }
func (b *blockingMessagingAdapter) SendMedia(context.Context, *ports.Media) error      { return nil }
func (b *blockingMessagingAdapter) SendTyping(context.Context, string, int) error { return nil }
func (b *blockingMessagingAdapter) SendSpeaking(context.Context, string, int) error { return nil }
func (b *blockingMessagingAdapter) SendVoice(context.Context, *models.Message) error { return nil }
func (b *blockingMessagingAdapter) HandleWebhook(context.Context, []byte) (*models.Message, error) {
	return nil, nil
}
func (b *blockingMessagingAdapter) GetUserInfo(context.Context, string) (*ports.UserInfo, error) {
	return nil, nil
}
func (b *blockingMessagingAdapter) React(context.Context, string, string) error { return nil }
func (b *blockingMessagingAdapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{}
}
func (b *blockingMessagingAdapter) ConvertAudioForPlatform(context.Context, []byte, string) ([]byte, string, error) {
	return nil, "", nil
}

func (b *blockingMessagingAdapter) Start(context.Context, func(context.Context, *models.Message)) error {
	b.startCalls.Add(1)
	close(b.entered)
	<-b.proceed
	return b.startErr
}

func TestStartMessagingAdapters_StartsBackgroundLoopsConcurrently(t *testing.T) {
	a := &App{}
	release := make(chan struct{})

	adapterA := &blockingMessagingAdapter{entered: make(chan struct{}), proceed: release}
	adapterB := &blockingMessagingAdapter{entered: make(chan struct{}), proceed: release}

	done := make(chan struct{})
	go func() {
		a.startMessagingAdapters(context.Background(), []namedMessagingAdapter{
			{channelType: "slack", adapter: adapterA},
			{channelType: "discord", adapter: adapterB},
		})
		close(done)
	}()

	select {
	case <-adapterA.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("adapter A did not start")
	}

	select {
	case <-adapterB.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("adapter B did not start concurrently")
	}

	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startMessagingAdapters did not finish after adapters were released")
	}

	if len(a.MessagingAdapters) != 2 {
		t.Fatalf("expected 2 started adapters, got %d", len(a.MessagingAdapters))
	}
	if got := adapterA.startCalls.Load(); got != 1 {
		t.Fatalf("expected adapter A start calls = 1, got %d", got)
	}
	if got := adapterB.startCalls.Load(); got != 1 {
		t.Fatalf("expected adapter B start calls = 1, got %d", got)
	}
}

func TestStartMessagingAdapters_TelegramUsesPluginStartLoop(t *testing.T) {
	a := &App{}
	release := make(chan struct{})
	adapter := &blockingMessagingAdapter{
		entered: make(chan struct{}),
		proceed: release,
	}

	done := make(chan struct{})
	go func() {
		a.startMessagingAdapters(context.Background(), []namedMessagingAdapter{
			{channelType: "telegram", adapter: adapter},
		})
		close(done)
	}()

	select {
	case <-adapter.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("telegram adapter did not start plugin loop")
	}

	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startMessagingAdapters did not finish after telegram adapter release")
	}

	if got := adapter.startCalls.Load(); got != 1 {
		t.Fatalf("expected telegram adapter start calls = 1, got %d", got)
	}
	if len(a.MessagingAdapters) != 1 {
		t.Fatalf("expected telegram adapter to be registered, got %d adapters", len(a.MessagingAdapters))
	}
}
