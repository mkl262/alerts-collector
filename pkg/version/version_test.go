// Copyright Contributors to the Open Cluster Management project

package version

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	Version = "1.2.3"
	GitCommit = "abc"
	BuildDate = "2026-01-02T03:04:05Z"

	got := String()
	for _, want := range []string{"version=1.2.3", "commit=abc", "built=2026-01-02T03:04:05Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String()=%q missing %q", got, want)
		}
	}
}
