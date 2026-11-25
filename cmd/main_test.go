// Copyright Contributors to the Open Cluster Management project

package main

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func TestLogLevelFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "debug", input: "debug"},
		{name: "info", input: "info"},
		{name: "warn", input: "warn"},
		{name: "error", input: "error"},
		{name: "default", input: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := logLevelFromString(tt.input)
			logger := level.NewFilter(log.NewNopLogger(), opt)
			if logger == nil {
				t.Fatalf("expected logger for level %q", tt.input)
			}
		})
	}
}
