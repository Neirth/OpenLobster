// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user_channel

import (
	"time"
)

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	row := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		row[j] = j
	}
	for i := 1; i <= len(a); i++ {
		row[0] = i
		prev := i - 1
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			ins := row[j] + 1
			del := row[j-1] + 1
			sub := prev + cost
			prev, row[j] = row[j], min3(ins, del, sub)
		}
	}
	return row[len(b)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// maxAllowedEdits implements max(1, min(3, needleLen/4)) per fuzzy-match policy.
func maxAllowedEdits(needleLen int) int {
	if needleLen <= 0 {
		return 0
	}
	inner := needleLen / 4
	if inner > 3 {
		inner = 3
	}
	if inner < 1 {
		return 1
	}
	return inner
}

type usernameCandidate struct {
	ChannelID       string
	PlatformUserID  string
	NormUsername    string
	LastSeen        time.Time
}

// pickBestUsernameMatch chooses the candidate with minimal Levenshtein distance to needle
// when distance <= maxAllowedEdits(len(needle)). Ties break by latest LastSeen.
func pickBestUsernameMatch(candidates []usernameCandidate, needle string) (channelType, platformUserID string, ok bool) {
	if len(candidates) == 0 || needle == "" {
		return "", "", false
	}
	maxDist := maxAllowedEdits(len(needle))
	bestDist := len(needle) + len(candidates[0].NormUsername) + 1
	var best usernameCandidate
	found := false
	for _, c := range candidates {
		d := levenshtein(needle, c.NormUsername)
		if d > maxDist {
			continue
		}
		if !found || d < bestDist || (d == bestDist && c.LastSeen.After(best.LastSeen)) {
			bestDist = d
			best = c
			found = true
		}
	}
	if !found {
		return "", "", false
	}
	return best.ChannelID, best.PlatformUserID, true
}
