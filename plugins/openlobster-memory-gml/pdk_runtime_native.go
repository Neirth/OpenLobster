// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"

func init() {
	pluginInputJSON = func(v interface{}) error { return pdk.InputJSON(v) }
	pluginOutputJSON = func(v interface{}) error { return pdk.OutputJSON(v) }
	pluginOutputString = func(s string) { pdk.OutputString(s) }
	pluginSetError = func(err error) { pdk.SetError(err) }
}
