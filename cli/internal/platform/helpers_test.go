package platform

import (
	"github.com/jamesatintegratnio/hctl/internal/testutil"
)

// ---------------------------------------------------------------------------
// fakeKubeClient — local alias for testutil.FakeKubeClient
// ---------------------------------------------------------------------------

// fakeKubeClient wraps testutil.FakeKubeClient for backward-compatible
// unexported field access in platform tests. New tests should prefer
// testutil.FakeKubeClient directly.
type fakeKubeClient = testutil.FakeKubeClient

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// contains checks whether sub is a substring of s.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
