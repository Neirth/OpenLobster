package main

import (
	"sync"

	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
)

var (
	hotConfigMu sync.RWMutex
	hotConfig   = map[string]interface{}{}
)

type hotConfigInput struct {
	Config map[string]interface{} `json:"config"`
}

func configureHot() int32 {
	var in hotConfigInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}

	hotConfigMu.Lock()
	hotConfig = cloneHotConfig(in.Config)
	hotConfigMu.Unlock()

	if err := pdk.OutputJSON(map[string]bool{"ok": true}); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func cloneHotConfig(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergedHotConfig(input map[string]interface{}) map[string]interface{} {
	out := cloneHotConfig(input)
	hotConfigMu.RLock()
	defer hotConfigMu.RUnlock()
	for k, v := range hotConfig {
		out[k] = v
	}
	return out
}
