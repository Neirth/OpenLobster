package serve

import (
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	msgrouter "github.com/neirth/openlobster/internal/infrastructure/adapters/messaging/router"
)

// initChannels initialises the channel registry and router.
// Actual messaging adapters are registered by initPlugins() after the plugins
// are loaded — this just sets up the routing infrastructure.
func (a *App) initChannels() {
	a.ChanReg = msgrouter.New()
	a.MsgRouter = msgrouter.NewRouter(a.ChanReg)
}

// rebuildActiveChannels returns a ChannelStatus list for all channels that
// currently have an active adapter registered in ChanReg.
func (a *App) rebuildActiveChannels() []dto.ChannelStatus {
	var list []dto.ChannelStatus
	for _, t := range a.ChanReg.ListTypes() {
		list = append(list, dto.ChannelStatus{
			ID: t, Name: t, Type: t, Status: "online",
			Enabled:      true,
			Capabilities: dto.ChannelCapabilities{HasTextStream: true},
		})
	}
	return list
}

// reloadChannel reconciles the runtime messaging wiring after channel-related
// config changes.
func (a *App) reloadChannel(_ string) {
	a.rebuildMessagingRuntime()
}
