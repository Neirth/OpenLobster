// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user_channel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_usernameContainsSQL_dialects(t *testing.T) {
	expr := "n"
	assert.Contains(t, usernameContainsSQL("sqlite", expr), "INSTR")
	assert.Contains(t, usernameContainsSQL("postgres", expr), "POSITION")
	assert.Contains(t, usernameContainsSQL("mysql", expr), "LOCATE")
}

func Test_levenshtein(t *testing.T) {
	assert.Equal(t, 0, levenshtein("", ""))
	assert.Equal(t, 3, levenshtein("foo", "foobar"))
	assert.Equal(t, 1, levenshtein("neirth", "neirt"))
	assert.Equal(t, 1, levenshtein("kitten", "sitten"))
}

func Test_pickBestUsernameMatch_exact(t *testing.T) {
	ts := time.Now()
	c := []usernameCandidate{
		{ChannelID: "telegram", PlatformUserID: "111", NormUsername: "other", LastSeen: ts},
		{ChannelID: "telegram", PlatformUserID: "222", NormUsername: "target", LastSeen: ts.Add(-time.Hour)},
	}
	ct, pid, ok := pickBestUsernameMatch(c, "target")
	assert.True(t, ok)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "222", pid)
}

func Test_pickBestUsernameMatch_typo(t *testing.T) {
	ts := time.Now()
	c := []usernameCandidate{
		{ChannelID: "telegram", PlatformUserID: "99", NormUsername: "verylonghandle", LastSeen: ts},
	}
	ct, pid, ok := pickBestUsernameMatch(c, "verylonghandlx")
	assert.True(t, ok)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "99", pid)
}

func Test_pickBestUsernameMatch_tieLastSeen(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// "testusex" -> both "testuser" and "testusez" at distance 1
	c := []usernameCandidate{
		{ChannelID: "telegram", PlatformUserID: "old", NormUsername: "testuser", LastSeen: t1},
		{ChannelID: "telegram", PlatformUserID: "new", NormUsername: "testusez", LastSeen: t2},
	}
	ct, pid, ok := pickBestUsernameMatch(c, "testusex")
	assert.True(t, ok)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "new", pid)
}

func Test_pickBestUsernameMatch_noMatch(t *testing.T) {
	c := []usernameCandidate{
		{ChannelID: "telegram", PlatformUserID: "1", NormUsername: "aaaa", LastSeen: time.Now()},
	}
	_, _, ok := pickBestUsernameMatch(c, "zzzzzzzz")
	assert.False(t, ok)
}
