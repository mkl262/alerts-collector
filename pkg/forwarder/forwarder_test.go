// Copyright Contributors to the Open Cluster Management project

package forwarder

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/template"
)

func TestForwardSucceedsWhenSomeAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	var okHits, failHits atomic.Uint64
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		okHits.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failHits.Add(1)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer fail.Close()

	fwder := newTestForwarder(t, ok.URL, fail.URL)
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err != nil {
		t.Fatalf("expected success when at least one alertmanager accepts alerts, got: %v", err)
	}
	if okHits.Load() != 1 {
		t.Fatalf("expected 1 hit on healthy alertmanager, got %d", okHits.Load())
	}
	if failHits.Load() != 1 {
		t.Fatalf("expected 1 hit on unavailable alertmanager, got %d", failHits.Load())
	}
}

func TestForwardFailsWhenAllAlertmanagersUnavailable(t *testing.T) {
	t.Parallel()

	failA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failA.Close()

	failB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failB.Close()

	fwder := newTestForwarder(t, failA.URL, failB.URL)
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err == nil {
		t.Fatal("expected error when all alertmanagers are unavailable")
	}
}

func TestForwardSucceedsWhenOneEndpointIsUnreachable(t *testing.T) {
	t.Parallel()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	fwder := newTestForwarder(t, ok.URL, "http://127.0.0.1:1")
	err := fwder.Forward(context.Background(), sampleAlerts())
	if err != nil {
		t.Fatalf("expected success when one endpoint is unreachable but another succeeds, got: %v", err)
	}
}

func TestForwardEmptyAlerts(t *testing.T) {
	t.Parallel()

	fwder := &Forwarder{logger: log.NewNopLogger()}
	if err := fwder.Forward(context.Background(), nil); err != nil {
		t.Fatalf("expected nil error for empty alerts, got: %v", err)
	}
}

func TestForwardAPIv1(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	fwder := &Forwarder{
		logger: log.NewNopLogger(),
		alertmanagers: []*Alertmanager{{
			logger:    log.NewNopLogger(),
			endpoints: []*url.URL{u},
			client:    &http.Client{Timeout: 2 * time.Second},
			timeout:   2 * time.Second,
			version:   APIv1,
		}},
		versions: []APIVersion{APIv1},
	}
	if err := fwder.Forward(context.Background(), sampleAlerts()); err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if gotPath != "/api/v1/alerts" {
		t.Fatalf("expected /api/v1/alerts, got %q", gotPath)
	}
}

func TestNewAlertmanager(t *testing.T) {
	t.Parallel()

	am, err := NewAlertmanager(log.NewNopLogger(), AlertmanagerConfig{
		EndpointsConfig: EndpointsConfig{
			StaticAddresses: []string{"alertmanager.example.com:9093"},
			Scheme:          "http",
			PathPrefix:      "/prefix",
		},
		APIVersion: APIv2,
		Timeout:    0,
	})
	if err != nil {
		t.Fatalf("NewAlertmanager: %v", err)
	}
	if len(am.endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(am.endpoints))
	}
	if am.endpoints[0].Host != "alertmanager.example.com:9093" {
		t.Fatalf("unexpected host: %q", am.endpoints[0].Host)
	}
	if am.endpoints[0].Path != "/prefix" {
		t.Fatalf("unexpected path: %q", am.endpoints[0].Path)
	}
}

func TestNewAlertmanagerRequiresEndpoints(t *testing.T) {
	t.Parallel()

	_, err := NewAlertmanager(log.NewNopLogger(), AlertmanagerConfig{})
	if err == nil {
		t.Fatal("expected error when endpoints are missing")
	}
}

func TestNewForwarder(t *testing.T) {
	t.Parallel()

	path := writeAlertingConfigFile(t, `alertmanagers:
  - static_configs:
      - alertmanager.example.com:9093
    scheme: http
    api_version: v2
    timeout: 2s
`)
	fwder, err := NewForwarder(log.NewNopLogger(), path)
	if err != nil {
		t.Fatalf("NewForwarder: %v", err)
	}
	if len(fwder.alertmanagers) != 1 {
		t.Fatalf("expected 1 alertmanager, got %d", len(fwder.alertmanagers))
	}
	if len(fwder.versions) != 1 || fwder.versions[0] != APIv2 {
		t.Fatalf("unexpected versions: %#v", fwder.versions)
	}
}

func TestNewForwarderEmptyConfig(t *testing.T) {
	t.Parallel()

	path := writeAlertingConfigFile(t, "alertmanagers: []\n")
	fwder, err := NewForwarder(log.NewNopLogger(), path)
	if err != nil {
		t.Fatalf("NewForwarder: %v", err)
	}
	if len(fwder.alertmanagers) != 0 {
		t.Fatalf("expected no alertmanagers, got %d", len(fwder.alertmanagers))
	}
}

func TestNewForwarderMissingFile(t *testing.T) {
	t.Parallel()

	_, err := NewForwarder(log.NewNopLogger(), "does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestPostAlertsDefaultTimeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	am := &Alertmanager{
		logger:  log.NewNopLogger(),
		client:  &http.Client{},
		timeout: 0,
	}
	if err := am.postAlerts(context.Background(), *u, strings.NewReader(`[]`)); err != nil {
		t.Fatalf("postAlerts: %v", err)
	}
}

func writeAlertingConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func newTestForwarder(t *testing.T, rawURLs ...string) *Forwarder {
	t.Helper()

	ams := make([]*Alertmanager, 0, len(rawURLs))
	for _, raw := range rawURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse url %q: %v", raw, err)
		}
		ams = append(ams, &Alertmanager{
			logger:    log.NewNopLogger(),
			endpoints: []*url.URL{u},
			client:    &http.Client{Timeout: 2 * time.Second},
			timeout:   2 * time.Second,
			version:   APIv2,
		})
	}

	return &Forwarder{
		logger:        log.NewNopLogger(),
		alertmanagers: ams,
		versions:      []APIVersion{APIv2},
	}
}

func sampleAlerts() template.Alerts {
	return template.Alerts{{
		Status:   "firing",
		Labels:   template.KV{"alertname": "TestAlert", "severity": "warning"},
		StartsAt: time.Date(2021, 4, 24, 7, 11, 41, 0, time.UTC),
	}}
}
