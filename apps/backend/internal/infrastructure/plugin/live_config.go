// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"fmt"

	"github.com/spf13/viper"
)

func cloneConfigMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func liveConfigForPlugin(pluginID string, fallback map[string]interface{}) map[string]interface{} {
	key := fmt.Sprintf("plugins.settings.%s", pluginID)
	if !viper.IsSet(key) {
		return cloneConfigMap(fallback)
	}
	return cloneConfigMap(viper.GetStringMap(key))
}
