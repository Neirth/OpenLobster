// Copyright (c) OpenLobster contributors. See LICENSE for details.

package runtime

import (
	"context"
	"encoding/json"
	"testing"

	pluginrpc "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/protocol"
)

func outputJSONFn(v any) Function {
	return func() int32 {
		if err := OutputJSON(v); err != nil {
			SetError(err)
			return 1
		}
		return 0
	}
}

func TestGetInfoUsesGetMetadataExport(t *testing.T) {
	svc := &service{
		plugin: Plugin{
			ID: "openlobster-test",
			Exports: map[string]Function{
				metadataExportName: outputJSONFn(map[string]any{
					"id":          "openlobster-metadata",
					"name":        "Metadata Plugin",
					"version":     "1.2.3",
					"description": "metadata-first",
					"type":        "memory",
					"schema": map[string]any{
						"type": "object",
					},
					"properties": map[string]any{
						"engine": "graph",
					},
				}),
			},
		},
		hub: newEventHub(),
	}

	got, err := svc.GetInfo(context.Background(), &pluginrpc.GetInfoRequest{})
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}
	if got.ID != "openlobster-metadata" {
		t.Fatalf("ID = %q, want openlobster-metadata", got.ID)
	}
	if got.Name != "Metadata Plugin" {
		t.Fatalf("Name = %q, want Metadata Plugin", got.Name)
	}
	if got.Version != "1.2.3" {
		t.Fatalf("Version = %q, want 1.2.3", got.Version)
	}
	if got.Type != "memory" {
		t.Fatalf("Type = %q, want memory", got.Type)
	}
	if !json.Valid(got.Schema) {
		t.Fatalf("Schema is not valid JSON: %q", string(got.Schema))
	}
	if !json.Valid(got.Properties) {
		t.Fatalf("Properties is not valid JSON: %q", string(got.Properties))
	}

	hasMetadata := false
	for _, fn := range got.Exports {
		if fn == metadataExportName {
			hasMetadata = true
			break
		}
	}
	if !hasMetadata {
		t.Fatalf("expected get_metadata export in GetInfo exports")
	}
}

func TestCallGetMetadataExport(t *testing.T) {
	svc := &service{
		plugin: Plugin{
			ID: "openlobster-virtual",
			Exports: map[string]Function{
				metadataExportName: outputJSONFn(map[string]any{
					"id":          "openlobster-virtual",
					"name":        "Virtual Plugin",
					"version":     "1.0.0",
					"description": "virtual metadata export",
					"type":        "secrets",
					"schema": map[string]any{
						"type": "object",
					},
					"properties": map[string]any{
						"backend": "json",
					},
				}),
			},
		},
		hub: newEventHub(),
	}

	resp, err := svc.Call(context.Background(), &pluginrpc.CallRequest{Function: metadataExportName})
	if err != nil {
		t.Fatalf("Call(get_metadata) error = %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Call(get_metadata) returned plugin error: %s", resp.Error)
	}

	var md Metadata
	if err := json.Unmarshal(resp.Output, &md); err != nil {
		t.Fatalf("invalid metadata JSON: %v", err)
	}
	if md.Name != "Virtual Plugin" {
		t.Fatalf("metadata.name = %q, want Virtual Plugin", md.Name)
	}
	if md.Type != "secrets" {
		t.Fatalf("metadata.type = %q, want secrets", md.Type)
	}
	if !json.Valid(md.Properties) {
		t.Fatalf("metadata.properties is not valid JSON: %q", string(md.Properties))
	}
}
