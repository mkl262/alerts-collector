// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/template"
)

func TestSanitizeKV(t *testing.T) {
	t.Parallel()

	got, err := sanitizeKV(template.KV{"alertname": "foo", "severity": "critical"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["alertname"] != "foo" || got["severity"] != "critical" {
		t.Fatalf("unexpected labels: %v", got)
	}
}

func TestSanitizeKVRejectsInvalidName(t *testing.T) {
	t.Parallel()

	_, err := sanitizeKV(template.KV{"": "foo"})
	if err == nil {
		t.Fatal("expected error for empty label name")
	}
}

func TestMarshalPostableAlertsV2OmitsEmptyGeneratorURL(t *testing.T) {
	t.Parallel()

	b, err := marshalPostableAlertsV2(template.Alerts{{
		Labels:   template.KV{"alertname": "foo"},
		StartsAt: mustParseTime(t, "2021-04-24T07:11:41Z"),
	}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(b)
	if strings.Contains(body, "generatorURL") {
		t.Fatalf("expected empty generatorURL to be omitted, got %s", body)
	}
	if !strings.Contains(body, `"alertname":"foo"`) {
		t.Fatalf("expected labels in payload, got %s", body)
	}
}

func mustParseTime(t *testing.T, raw string) (out time.Time) {
	t.Helper()
	out, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return out
}
