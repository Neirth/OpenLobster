// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import "testing"

func makeResolveInput(channelID, senderID string, metadata map[string]interface{}, config map[string]interface{}) resolveChannelIDInput {
	input := resolveChannelIDInput{Config: config}
	input.Message.ChannelID = channelID
	input.Message.SenderID = senderID
	input.Message.Metadata = metadata
	return input
}

func TestResolveDiscordDestination_UsesExplicitChannelID(t *testing.T) {
	input := makeResolveInput("123456", "999999", nil, map[string]interface{}{})

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.ChannelID != "123456" {
		t.Fatalf("expected ChannelID=123456, got %q", dst.ChannelID)
	}
	if dst.RecipientID != "999999" {
		t.Fatalf("expected RecipientID=999999, got %q", dst.RecipientID)
	}
}

func TestResolveDiscordDestination_UsesMetadataPlatformChannelID(t *testing.T) {
	input := makeResolveInput(
		"discord",
		"",
		map[string]interface{}{"platform_channel_id": " 777777 "},
		map[string]interface{}{},
	)

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.ChannelID != "777777" {
		t.Fatalf("expected ChannelID=777777, got %q", dst.ChannelID)
	}
}

func TestResolveDiscordDestination_UsesDefaultChannelID(t *testing.T) {
	input := makeResolveInput("", "", nil, map[string]interface{}{"default_channel_id": " 888888 "})

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.ChannelID != "888888" {
		t.Fatalf("expected ChannelID=888888, got %q", dst.ChannelID)
	}
}

func TestResolveDiscordDestination_UsesSenderAsDMRecipient(t *testing.T) {
	input := makeResolveInput("", "555555", nil, map[string]interface{}{})

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.ChannelID != "" {
		t.Fatalf("expected empty ChannelID, got %q", dst.ChannelID)
	}
	if dst.RecipientID != "555555" {
		t.Fatalf("expected RecipientID=555555, got %q", dst.RecipientID)
	}
}

func TestResolveDiscordDestination_UsesMetadataUserAsDMRecipient(t *testing.T) {
	input := makeResolveInput(
		"",
		"",
		map[string]interface{}{"platform_user_id": " 666666 "},
		map[string]interface{}{},
	)

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.RecipientID != "666666" {
		t.Fatalf("expected RecipientID=666666, got %q", dst.RecipientID)
	}
}

func TestResolveDiscordDestination_UsesDefaultRecipientID(t *testing.T) {
	input := makeResolveInput("", "", nil, map[string]interface{}{"default_recipient_id": " 444444 "})

	dst, err := resolveDiscordDestination(input)
	if err != nil {
		t.Fatalf("resolveDiscordDestination returned error: %v", err)
	}
	if dst.RecipientID != "444444" {
		t.Fatalf("expected RecipientID=444444, got %q", dst.RecipientID)
	}
}

func TestResolveDiscordDestination_ErrorWhenMissingDestination(t *testing.T) {
	input := makeResolveInput("", "", nil, map[string]interface{}{})

	_, err := resolveDiscordDestination(input)
	if err == nil {
		t.Fatal("expected error when destination data is missing")
	}
}

func TestResolveDiscordChannelID_ReturnsRecipientForDMOnlyDestination(t *testing.T) {
	input := makeResolveInput("", "555555", nil, map[string]interface{}{})

	resolved, err := resolveDiscordChannelID(input)
	if err != nil {
		t.Fatalf("resolveDiscordChannelID returned error: %v", err)
	}
	if resolved != "555555" {
		t.Fatalf("expected resolved destination 555555, got %q", resolved)
	}
}

func TestReadStringMap_TrimsAndRejectsNonString(t *testing.T) {
	m := map[string]interface{}{
		"ok":    "  value  ",
		"bad":   123,
		"empty": "   ",
	}

	if got := readStringMap(m, "ok"); got != "value" {
		t.Fatalf("expected trimmed string 'value', got %q", got)
	}
	if got := readStringMap(m, "bad"); got != "" {
		t.Fatalf("expected empty string for non-string value, got %q", got)
	}
	if got := readStringMap(m, "empty"); got != "" {
		t.Fatalf("expected empty string for blank value, got %q", got)
	}
	if got := readStringMap(m, "missing"); got != "" {
		t.Fatalf("expected empty string for missing key, got %q", got)
	}
}

func TestIsDiscordUnknownChannelError(t *testing.T) {
	if isDiscordUnknownChannelError(nil) {
		t.Fatal("expected false for nil error")
	}
	if !isDiscordUnknownChannelError(errString("HTTP 404: Unknown Channel")) {
		t.Fatal("expected true for unknown channel error")
	}
	if isDiscordUnknownChannelError(errString("request timed out")) {
		t.Fatal("expected false for unrelated errors")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
