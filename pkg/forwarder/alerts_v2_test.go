// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"strings"
	"testing"

	"github.com/prometheus/alertmanager/template"
)

func TestTemplateAlertsToPostableV2WithAnnotations(t *testing.T) {
	t.Parallel()

	got, err := templateAlertsToPostableV2(template.Alerts{{
		Labels:       template.KV{"alertname": "foo"},
		Annotations:  template.KV{"summary": "something broke"},
		GeneratorURL: "http://prometheus.example.com/graph",
		StartsAt:     mustParseTime(t, "2021-04-24T07:11:41Z"),
		EndsAt:       mustParseTime(t, "2021-04-24T08:11:41Z"),
	}})
	if err != nil {
		t.Fatalf("templateAlertsToPostableV2: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(got))
	}
	if got[0].Annotations["summary"] != "something broke" {
		t.Fatalf("unexpected annotations: %#v", got[0].Annotations)
	}
	if got[0].GeneratorURL != "http://prometheus.example.com/graph" {
		t.Fatalf("unexpected generatorURL: %q", got[0].GeneratorURL)
	}
}

func TestTemplateAlertsToPostableV2RejectsInvalidLabels(t *testing.T) {
	t.Parallel()

	_, err := templateAlertsToPostableV2(template.Alerts{{
		Labels: template.KV{"": "bad"},
	}})
	if err == nil {
		t.Fatal("expected error for invalid labels")
	}
	if !strings.Contains(err.Error(), "alert labels") {
		t.Fatalf("expected labels error, got %v", err)
	}
}

func TestTemplateAlertsToPostableV2RejectsInvalidAnnotations(t *testing.T) {
	t.Parallel()

	_, err := templateAlertsToPostableV2(template.Alerts{{
		Labels:      template.KV{"alertname": "foo"},
		Annotations: template.KV{"": "bad"},
	}})
	if err == nil {
		t.Fatal("expected error for invalid annotations")
	}
	if !strings.Contains(err.Error(), "alert annotations") {
		t.Fatalf("expected annotations error, got %v", err)
	}
}

func TestMarshalPostableAlertsV2RejectsInvalidAlerts(t *testing.T) {
	t.Parallel()

	_, err := marshalPostableAlertsV2(template.Alerts{{
		Labels: template.KV{"": "bad"},
	}})
	if err == nil {
		t.Fatal("expected marshal error for invalid alerts")
	}
}
