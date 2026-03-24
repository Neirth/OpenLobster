// Copyright (c) OpenLobster contributors. See LICENSE for details.

package dockermodelrunner

import (
	"testing"
)

func TestDefaultEndpoint(t *testing.T) {
	if DefaultEndpoint != "http://host.docker.internal:12434/engines/v1" {
		t.Errorf("DefaultEndpoint = %q, want http://host.docker.internal:12434/engines/v1", DefaultEndpoint)
	}
}

func TestNewAdapter_UsesDefaultEndpointWhenEmpty(t *testing.T) {
	a := NewAdapter("", "ai/mistral-nemo", 500, "medium")
	if a == nil {
		t.Fatal("NewAdapter returned nil")
	}
}

func TestNewAdapter_UsesProvidedEndpoint(t *testing.T) {
	a := NewAdapter("http://host.docker.internal:12434/engines/v1", "ai/llama3.2", 500, "medium")
	if a == nil {
		t.Fatal("NewAdapter returned nil")
	}
}
