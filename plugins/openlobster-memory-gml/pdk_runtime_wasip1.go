//go:build wasip1

package main

import pdk "github.com/extism/go-pdk"

func init() {
	pluginInputJSON = func(v interface{}) error { return pdk.InputJSON(v) }
	pluginOutputJSON = func(v interface{}) error { return pdk.OutputJSON(v) }
	pluginOutputString = func(s string) { pdk.OutputString(s) }
	pluginSetError = func(err error) { pdk.SetError(err) }
}
