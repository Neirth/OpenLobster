package ipc

import (
	"testing"
	"time"
)

func TestEffectiveCallTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		function   string
		configured time.Duration
		want       time.Duration
	}{
		{
			name:       "chat enforces minimum timeout",
			function:   "chat",
			configured: 10 * time.Second,
			want:       minLongRunningCallTimeout,
		},
		{
			name:       "tts enforces minimum timeout",
			function:   "tts",
			configured: 30 * time.Second,
			want:       minLongRunningCallTimeout,
		},
		{
			name:       "non long-running keeps configured timeout",
			function:   "send",
			configured: 10 * time.Second,
			want:       10 * time.Second,
		},
		{
			name:       "chat keeps larger configured timeout",
			function:   "chat",
			configured: 3 * time.Minute,
			want:       3 * time.Minute,
		},
		{
			name:       "non long-running uses default when configured is zero",
			function:   "send",
			configured: 0,
			want:       10 * time.Second,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveCallTimeout(tc.function, tc.configured)
			if got != tc.want {
				t.Fatalf("effectiveCallTimeout(%q, %s) = %s, want %s", tc.function, tc.configured, got, tc.want)
			}
		})
	}
}
