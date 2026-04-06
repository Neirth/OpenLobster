// Copyright (c) OpenLobster contributors. See LICENSE for details.

package protocol

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const CodecName = "json"

// JSONCodec lets us run gRPC without protobuf codegen for plugin IPC.
type JSONCodec struct{}

func (JSONCodec) Name() string {
	return CodecName
}

func (JSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (JSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(JSONCodec{})
}
